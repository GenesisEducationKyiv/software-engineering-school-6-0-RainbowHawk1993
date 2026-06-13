package api

import (
	"net/http"
	"time"

	"releasesapi/internal/platform/metrics"

	"github.com/go-chi/chi/v5"
)

func metricsMiddleware(serviceMetrics *metrics.ServiceMetrics) func(http.Handler) http.Handler {
	if serviceMetrics == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(recorder, r)

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}

			serviceMetrics.ObserveHTTPRequest(r.Method, route, recorder.status, time.Since(startedAt))
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
