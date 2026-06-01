//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"releasesapi/internal/api"
	"releasesapi/internal/mailer"
	appmetrics "releasesapi/internal/metrics"
	"releasesapi/internal/migrations"
	"releasesapi/internal/service"
	"releasesapi/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

var (
	testAPIKey      = "test-api-key-12345"
	testDatabaseURL string
	testRedisAddr   string
	testAppBaseURL  = "http://localhost:8080"
)

type mockGitHubClient struct{}

func (m *mockGitHubClient) RepoExists(ctx context.Context, owner, repo string) error {
	return nil
}

func (m *mockGitHubClient) LatestReleaseTag(ctx context.Context, owner, repo string) (string, error) {
	return "v1.0.0", nil
}

type mockMailer struct{}

func (m *mockMailer) Send(ctx context.Context, msg mailer.Message) error {
	return nil
}

func init() {
	testDatabaseURL = os.Getenv("TEST_DATABASE_URL")
	if testDatabaseURL == "" {
		testDatabaseURL = "postgres://postgres:postgres@localhost:5435/releases?sslmode=disable"
	}

	testRedisAddr = os.Getenv("TEST_REDIS_ADDR")
	if testRedisAddr == "" {
		testRedisAddr = "localhost:6379"
	}
}

type TestServer struct {
	client  *http.Client
	baseURL string
	apiKey  string
	db      *pgxpool.Pool
	redis   *redis.Client
	server  *httptest.Server
	logger  *log.Logger
}

func setupTestDB(ctx context.Context) (*pgxpool.Pool, error) {
	db, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
		return nil, err
	}

	if err := migrations.Run(ctx, db); err != nil {
		return nil, err
	}

	return db, nil
}

func setupTestRedis(ctx context.Context) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: testRedisAddr,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

func setupTestServer(t *testing.T) *TestServer {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := setupTestDB(ctx)
	if err != nil {
		t.Fatalf("failed to setup test database: %v", err)
	}

	redis, err := setupTestRedis(ctx)
	if err != nil {
		t.Fatalf("failed to setup test redis: %v", err)
	}

	if err := redis.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("failed to flush redis: %v", err)
	}

	if _, err := db.Exec(ctx, "TRUNCATE TABLE subscriptions"); err != nil {
		t.Fatalf("failed to truncate subscriptions: %v", err)
	}

	logger := log.New(io.Discard, "", 0)
	registry, serviceMetrics := appmetrics.NewRegistry()
	subscriptionStore := store.NewPostgresSubscriptionStore(db)
	githubClient := &mockGitHubClient{}
	smtpMailer := &mockMailer{}
	notificationBuilder := &mailer.DefaultNotificationBuilder{}
	subscriptionService := service.NewSubscriptionService(
		subscriptionStore,
		githubClient,
		smtpMailer,
		notificationBuilder,
		testAppBaseURL,
	)

	handler := api.NewHandler(subscriptionService)
	router := api.NewRouter(handler, logger, serviceMetrics, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), testAPIKey)

	server := httptest.NewServer(router)

	return &TestServer{
		client:  server.Client(),
		baseURL: server.URL,
		apiKey:  testAPIKey,
		db:      db,
		redis:   redis,
		server:  server,
		logger:  logger,
	}
}

func (ts *TestServer) cleanup(t *testing.T) {
	ts.server.Close()
	if ts.db != nil {
		ts.db.Close()
	}
	if ts.redis != nil {
		_ = ts.redis.Close()
	}
}

func (ts *TestServer) get(path string, requireAuth bool) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, ts.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if requireAuth {
		req.Header.Set("X-API-Key", ts.apiKey)
	}
	return ts.client.Do(req)
}

func (ts *TestServer) post(path string, body interface{}, requireAuth bool) (*http.Response, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, ts.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if requireAuth {
		req.Header.Set("X-API-Key", ts.apiKey)
	}
	return ts.client.Do(req)
}

func (ts *TestServer) getTokenFromDB(email, repo string) (confirmToken, unsubscribeToken string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var ct, ut string
	err = ts.db.QueryRow(ctx, `
		SELECT confirm_token, unsubscribe_token
		FROM subscriptions
		WHERE email = $1 AND repo_owner || '/' || repo_name = $2
		LIMIT 1
	`, email, repo).Scan(&ct, &ut)

	return ct, ut, err
}

func TestSubscribeSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, err := ts.post("/api/subscribe", api.SubscribeRequest{
		Email: "test@example.com",
		Repo:  "golang/go",
	}, true)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	var msgResp api.MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(msgResp.Message, "confirmation email sent") {
		t.Fatalf("unexpected message: %s", msgResp.Message)
	}
}

func TestSubscribeInvalidRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, err := ts.post("/api/subscribe", map[string]string{"invalid": "data"}, true)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestSubscribeMissingAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, err := ts.post("/api/subscribe", api.SubscribeRequest{
		Email: "test@example.com",
		Repo:  "golang/go",
	}, false)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestSubscribeWrongAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	req, err := http.NewRequest(http.MethodPost, ts.baseURL+"/api/subscribe", bytes.NewReader([]byte(`{"email":"test@example.com","repo":"golang/go"}`)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "wrong-key")

	resp, err := ts.client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestConfirmToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, _ := ts.post("/api/subscribe", api.SubscribeRequest{Email: "test@example.com", Repo: "golang/go"}, true)
	resp.Body.Close()

	confirmToken, _, _ := ts.getTokenFromDB("test@example.com", "golang/go")
	resp, err := ts.get(fmt.Sprintf("/api/confirm/%s", confirmToken), true)
	if err != nil {
		t.Fatalf("confirm request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestConfirmInvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, _ := ts.get("/api/confirm/0000000000000000000000000000000000000000000000000000000000000000", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestUnsubscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, _ := ts.post("/api/subscribe", api.SubscribeRequest{Email: "test@example.com", Repo: "golang/go"}, true)
	resp.Body.Close()

	_, unsubscribeToken, _ := ts.getTokenFromDB("test@example.com", "golang/go")
	resp, err := ts.get(fmt.Sprintf("/api/unsubscribe/%s", unsubscribeToken), true)
	if err != nil {
		t.Fatalf("unsubscribe request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestUnsubscribeInvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, _ := ts.get("/api/unsubscribe/0000000000000000000000000000000000000000000000000000000000000000", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestListSubscriptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, _ := ts.post("/api/subscribe", api.SubscribeRequest{Email: "test@example.com", Repo: "golang/go"}, true)
	resp.Body.Close()

	confirmToken, _, _ := ts.getTokenFromDB("test@example.com", "golang/go")
	resp, _ = ts.get(fmt.Sprintf("/api/confirm/%s", confirmToken), true)
	resp.Body.Close()

	resp, err := ts.get("/api/subscriptions?email=test@example.com", true)
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var subscriptions []api.SubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&subscriptions); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subscriptions))
	}
}

func TestListSubscriptionsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, _ := ts.get("/api/subscriptions?email=nonexistent@example.com", true)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	var subscriptions []api.SubscriptionResponse
	_ = json.NewDecoder(resp.Body).Decode(&subscriptions)
	if len(subscriptions) != 0 {
		t.Fatalf("expected 0 subscriptions, got %d", len(subscriptions))
	}
}

func TestListUISubscriptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, _ := ts.post("/api/subscribe", api.SubscribeRequest{Email: "test@example.com", Repo: "golang/go"}, true)
	resp.Body.Close()

	confirmToken, _, _ := ts.getTokenFromDB("test@example.com", "golang/go")
	resp, _ = ts.get(fmt.Sprintf("/api/confirm/%s", confirmToken), true)
	resp.Body.Close()

	resp, err := ts.get("/ui/subscriptions?email=test@example.com", true)
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var subscriptions []api.UISubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&subscriptions); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subscriptions))
	}
}

func TestHomeEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, err := ts.get("/", false)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	resp, err := ts.get("/metrics", true)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}
