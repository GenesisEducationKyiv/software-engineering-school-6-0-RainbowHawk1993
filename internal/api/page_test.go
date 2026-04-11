package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomeServesHTMLPage(t *testing.T) {
	t.Parallel()

	handler := NewHandler(nil)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.Home(recorder, request)

	response := recorder.Result()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected text/html content type, got %q", contentType)
	}
	if !strings.Contains(recorder.Body.String(), "Create Subscription") {
		t.Fatalf("expected subscription page content, got %q", recorder.Body.String())
	}
}
