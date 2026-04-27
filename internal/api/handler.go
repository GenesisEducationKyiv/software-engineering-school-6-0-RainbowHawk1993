package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"releasesapi/internal/apperr"
	appmetrics "releasesapi/internal/metrics"
	"releasesapi/internal/model"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type SubscriptionUseCase interface {
	Subscribe(rctx context.Context, email, repo string) (model.Subscription, error)
	Confirm(ctx context.Context, token string) (model.Subscription, error)
	Unsubscribe(ctx context.Context, token string) error
	ListByEmail(ctx context.Context, email string) ([]model.Subscription, error)
}

type Handler struct {
	subscriptions SubscriptionUseCase
}

type subscribeRequest struct {
	Email string `json:"email"`
	Repo  string `json:"repo"`
}

type messageResponse struct {
	Message string `json:"message"`
}

type subscriptionResponse struct {
	Email       string `json:"email"`
	Repo        string `json:"repo"`
	Confirmed   bool   `json:"confirmed"`
	LastSeenTag string `json:"last_seen_tag"`
}

type uiSubscriptionResponse struct {
	Email            string `json:"email"`
	Repo             string `json:"repo"`
	Confirmed        bool   `json:"confirmed"`
	LastSeenTag      string `json:"last_seen_tag"`
	UnsubscribeToken string `json:"unsubscribe_token"`
}

func NewHandler(subscriptions SubscriptionUseCase) *Handler {
	return &Handler{subscriptions: subscriptions}
}

func NewRouter(handler *Handler, logger *log.Logger, metrics *appmetrics.ServiceMetrics, metricsHandler http.Handler, apiKey string) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(metricsMiddleware(metrics))
	router.Get("/", handler.Home)
	router.With(apiKeyMiddleware(apiKey)).Get("/ui/subscriptions", handler.ListUISubscriptions)
	if metricsHandler != nil {
		router.With(apiKeyMiddleware(apiKey)).Handle("/metrics", metricsHandler)
	}

	router.Route("/api", func(r chi.Router) {
		r.Use(apiKeyMiddleware(apiKey))
		r.Post("/subscribe", handler.Subscribe)
		r.Get("/confirm/{token}", handler.Confirm)
		r.Get("/unsubscribe/{token}", handler.Unsubscribe)
		r.Get("/subscriptions", handler.ListSubscriptions)
	})

	if logger != nil {
		logger.Printf("api router configured")
	}

	return router
}

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var request subscribeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, messageResponse{Message: "invalid request body"})
		return
	}

	if _, err := h.subscriptions.Subscribe(r.Context(), request.Email, request.Repo); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, messageResponse{Message: "subscription created; confirmation email sent"})
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if _, err := h.subscriptions.Confirm(r.Context(), token); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, messageResponse{Message: "subscription confirmed successfully"})
}

func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if err := h.subscriptions.Unsubscribe(r.Context(), token); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, messageResponse{Message: "unsubscribed successfully"})
}

func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	subscriptions, err := h.subscriptions.ListByEmail(r.Context(), email)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	response := make([]subscriptionResponse, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		response = append(response, toSubscriptionResponse(subscription))
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ListUISubscriptions(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	subscriptions, err := h.subscriptions.ListByEmail(r.Context(), email)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	response := make([]uiSubscriptionResponse, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		response = append(response, toUISubscriptionResponse(subscription))
	}

	writeJSON(w, http.StatusOK, response)
}

func toSubscriptionResponse(subscription model.Subscription) subscriptionResponse {
	return subscriptionResponse{
		Email:       subscription.Email,
		Repo:        subscription.Repo(),
		Confirmed:   subscription.Confirmed,
		LastSeenTag: subscription.LastSeenTag,
	}
}

func toUISubscriptionResponse(subscription model.Subscription) uiSubscriptionResponse {
	return uiSubscriptionResponse{
		Email:            subscription.Email,
		Repo:             subscription.Repo(),
		Confirmed:        subscription.Confirmed,
		LastSeenTag:      subscription.LastSeenTag,
		UnsubscribeToken: subscription.UnsubscribeToken,
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"

	switch {
	case errors.Is(err, apperr.ErrInvalidEmail), errors.Is(err, apperr.ErrInvalidRepoFormat), errors.Is(err, apperr.ErrInvalidToken):
		status = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, apperr.ErrRepoNotFound), errors.Is(err, apperr.ErrTokenNotFound):
		status = http.StatusNotFound
		message = err.Error()
	case errors.Is(err, apperr.ErrAlreadySubscribed):
		status = http.StatusConflict
		message = err.Error()
	case errors.Is(err, apperr.ErrRateLimited):
		status = http.StatusServiceUnavailable
		message = "github api rate limit reached"
	}

	writeJSON(w, status, messageResponse{Message: message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
