package api

import (
	"encoding/json"
	"log"
	"net/http"

	"releasesapi/internal/modules/subscription/domain"
	"releasesapi/internal/modules/subscription/ports"
	"releasesapi/internal/platform/apperr"
	"releasesapi/internal/platform/metrics"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	subscriptions ports.UseCase
}

type SubscribeRequest struct {
	Email string `json:"email"`
	Repo  string `json:"repo"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type SubscriptionResponse struct {
	Email       string `json:"email"`
	Repo        string `json:"repo"`
	Confirmed   bool   `json:"confirmed"`
	LastSeenTag string `json:"last_seen_tag"`
}

type UISubscriptionResponse struct {
	Email            string `json:"email"`
	Repo             string `json:"repo"`
	Confirmed        bool   `json:"confirmed"`
	LastSeenTag      string `json:"last_seen_tag"`
	UnsubscribeToken string `json:"unsubscribe_token"`
}

func NewHandler(subscriptions ports.UseCase) *Handler {
	return &Handler{subscriptions: subscriptions}
}

func NewRouter(handler *Handler, logger *log.Logger, serviceMetrics *metrics.ServiceMetrics, metricsHandler http.Handler, apiKey string) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(metricsMiddleware(serviceMetrics))

	registerUIRoutes(router, handler, apiKey)
	registerAPIRoutes(router, handler, apiKey)
	registerMetricsRoutes(router, metricsHandler, apiKey)

	if logger != nil {
		logger.Printf("api router configured")
	}

	return router
}

func registerUIRoutes(r chi.Router, h *Handler, apiKey string) {
	r.Get("/", h.Home)
	r.With(apiKeyMiddleware(apiKey)).Get("/ui/subscriptions", h.ListUISubscriptions)
}

func registerAPIRoutes(r chi.Router, h *Handler, apiKey string) {
	r.Route("/api", func(r chi.Router) {
		r.Use(apiKeyMiddleware(apiKey))
		r.Post("/subscribe", h.Subscribe)
		r.Get("/confirm/{token}", h.Confirm)
		r.Get("/unsubscribe/{token}", h.Unsubscribe)
		r.Get("/subscriptions", h.ListSubscriptions)
	})
}

func registerMetricsRoutes(r chi.Router, h http.Handler, apiKey string) {
	if h != nil {
		r.With(apiKeyMiddleware(apiKey)).Handle("/metrics", h)
	}
}

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var request SubscribeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, MessageResponse{Message: "invalid request body"})
		return
	}

	if _, err := h.subscriptions.Subscribe(r.Context(), request.Email, request.Repo); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{Message: "subscription created; confirmation email sent"})
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if _, err := h.subscriptions.Confirm(r.Context(), token); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{Message: "subscription confirmed successfully"})
}

func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if err := h.subscriptions.Unsubscribe(r.Context(), token); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{Message: "unsubscribed successfully"})
}

func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	subscriptions, err := h.subscriptions.ListByEmail(r.Context(), email)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	response := make([]SubscriptionResponse, 0, len(subscriptions))
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

	response := make([]UISubscriptionResponse, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		response = append(response, toUISubscriptionResponse(subscription))
	}

	writeJSON(w, http.StatusOK, response)
}

func toSubscriptionResponse(subscription domain.Subscription) SubscriptionResponse {
	return SubscriptionResponse{
		Email:       subscription.Email,
		Repo:        subscription.Repo(),
		Confirmed:   subscription.Confirmed,
		LastSeenTag: subscription.LastSeenTag,
	}
}

func toUISubscriptionResponse(subscription domain.Subscription) UISubscriptionResponse {
	return UISubscriptionResponse{
		Email:            subscription.Email,
		Repo:             subscription.Repo(),
		Confirmed:        subscription.Confirmed,
		LastSeenTag:      subscription.LastSeenTag,
		UnsubscribeToken: subscription.UnsubscribeToken,
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(apperr.AppError); ok {
		writeJSON(w, appErr.HTTPStatus(), MessageResponse{Message: appErr.Error()})
		return
	}

	writeJSON(w, http.StatusInternalServerError, MessageResponse{Message: "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
