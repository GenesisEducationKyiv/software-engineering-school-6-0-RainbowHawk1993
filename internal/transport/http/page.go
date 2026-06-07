package api

import (
	"embed"
	"net/http"
)

//go:embed web/index.html
var webFiles embed.FS

func (h *Handler) Home(w http.ResponseWriter, _ *http.Request) {
	page, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "page unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(page)
}
