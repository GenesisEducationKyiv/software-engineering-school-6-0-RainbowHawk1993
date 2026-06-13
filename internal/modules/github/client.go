package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"releasesapi/internal/platform/apperr"
	"releasesapi/internal/platform/metrics"
)

const baseURL = "https://api.github.com"

type Client struct {
	baseURL    string
	token      string
	cache      Cache
	metrics    *metrics.ServiceMetrics
	httpClient *http.Client
	logger     *log.Logger
}

func NewClient(token string, cache Cache, serviceMetrics *metrics.ServiceMetrics, logger *log.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		cache:   cache,
		metrics: serviceMetrics,
		logger:  logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) RepoExists(ctx context.Context, owner, repo string) error {
	if c.cache != nil {
		exists, err := c.cache.GetRepoExists(ctx, owner, repo)
		switch {
		case err == nil && exists:
			c.observeGitHub("repo_exists", "cache", "hit_exists")
			return nil
		case err == nil && !exists:
			c.observeGitHub("repo_exists", "cache", "hit_not_found")
			return apperr.ErrRepoNotFound
		case errors.Is(err, errCacheMiss):
			c.observeGitHub("repo_exists", "cache", "miss")
		case err != nil:
			c.observeGitHub("repo_exists", "cache", "error")
		}
	}

	response, err := c.doRequest(ctx, fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo))
	if err != nil {
		return err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			c.logger.Printf("failed to close response body: %v", err)
		}
	}()

	switch {
	case response.StatusCode == http.StatusOK:
		c.cacheRepoExists(ctx, owner, repo, true)
		c.observeGitHub("repo_exists", "api", "ok")
		return nil
	case isRateLimited(response):
		c.observeGitHub("repo_exists", "api", "rate_limited")
		return apperr.ErrRateLimited
	case response.StatusCode == http.StatusNotFound:
		c.cacheRepoExists(ctx, owner, repo, false)
		c.observeGitHub("repo_exists", "api", "not_found")
		return apperr.ErrRepoNotFound
	default:
		c.observeGitHub("repo_exists", "api", "error")
		return unexpectedStatus(response)
	}
}

func (c *Client) LatestReleaseTag(ctx context.Context, owner, repo string) (string, error) {
	if c.cache != nil {
		tag, found, err := c.cache.GetLatestReleaseTag(ctx, owner, repo)
		switch {
		case err == nil && found:
			c.observeGitHub("latest_release", "cache", "hit_release")
			return tag, nil
		case err == nil && !found:
			c.observeGitHub("latest_release", "cache", "hit_empty")
			return "", nil
		case errors.Is(err, errCacheMiss):
			c.observeGitHub("latest_release", "cache", "miss")
		case err != nil:
			c.observeGitHub("latest_release", "cache", "error")
		}
	}

	response, err := c.doRequest(ctx, fmt.Sprintf("%s/repos/%s/%s/releases?per_page=1", c.baseURL, owner, repo))
	if err != nil {
		return "", err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			c.logger.Printf("failed to close response body: %v", err)
		}
	}()

	switch {
	case response.StatusCode == http.StatusOK:
		var releases []struct {
			TagName string `json:"tag_name"`
		}

		if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
			c.observeGitHub("latest_release", "api", "error")
			return "", err
		}
		if len(releases) == 0 {
			c.cacheLatestRelease(ctx, owner, repo, "", false)
			c.observeGitHub("latest_release", "api", "empty")
			return "", nil
		}

		c.cacheLatestRelease(ctx, owner, repo, releases[0].TagName, true)
		c.observeGitHub("latest_release", "api", "ok")
		return releases[0].TagName, nil
	case isRateLimited(response):
		c.observeGitHub("latest_release", "api", "rate_limited")
		return "", apperr.ErrRateLimited
	case response.StatusCode == http.StatusNotFound:
		c.observeGitHub("latest_release", "api", "not_found")
		return "", apperr.ErrRepoNotFound
	default:
		c.observeGitHub("latest_release", "api", "error")
		return "", unexpectedStatus(response)
	}
}

func (c *Client) doRequest(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "releases-api")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(request)
}

func isRateLimited(response *http.Response) bool {
	if response.StatusCode == http.StatusTooManyRequests {
		return true
	}

	return response.StatusCode == http.StatusForbidden && response.Header.Get("X-RateLimit-Remaining") == "0"
}

func unexpectedStatus(response *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(response.Body, 512))
	if err != nil {
		return fmt.Errorf("github unexpected status %d", response.StatusCode)
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("github unexpected status %d", response.StatusCode)
	}

	return fmt.Errorf("github unexpected status %d: %s", response.StatusCode, message)
}

func (c *Client) cacheRepoExists(ctx context.Context, owner, repo string, exists bool) {
	if c.cache == nil {
		return
	}
	_ = c.cache.SetRepoExists(ctx, owner, repo, exists)
}

func (c *Client) cacheLatestRelease(ctx context.Context, owner, repo, tag string, found bool) {
	if c.cache == nil {
		return
	}
	_ = c.cache.SetLatestReleaseTag(ctx, owner, repo, tag, found)
}

func (c *Client) observeGitHub(operation, source, outcome string) {
	if c.metrics == nil {
		return
	}
	c.metrics.ObserveGitHubRequest(operation, source, outcome)
}
