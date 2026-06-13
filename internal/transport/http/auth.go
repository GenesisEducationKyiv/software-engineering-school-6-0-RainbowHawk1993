package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const apiKeyHeader = "X-API-Key"

func apiKeyMiddleware(expectedKey string) func(http.Handler) http.Handler {
	trimmedKey := strings.TrimSpace(expectedKey)
	if trimmedKey == "" {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validAPIKey(r.Header.Get(apiKeyHeader), trimmedKey) {
				writeJSON(w, http.StatusUnauthorized, MessageResponse{Message: "missing or invalid api key"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validAPIKey(provided, expected string) bool {
	if expected == "" {
		return true
	}

	provided = strings.TrimSpace(provided)
	if len(provided) != len(expected) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
