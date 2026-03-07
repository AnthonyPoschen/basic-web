package util

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/CAFxX/httpcompression"
)

type statusWriter struct {
	http.ResponseWriter
	Status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.Status = code
	sw.ResponseWriter.WriteHeader(code)
}
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if w.Header().Get("Cache-Control") == "" {
			if IsDev() {
				w.Header().Set("Cache-Control", "no-cache")
			} else {
				w.Header().Set("Cache-Control", "max-age=86400") // 1 day cache expiry
			}
		}
		sw := &statusWriter{ResponseWriter: w, Status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)

		slog.Debug("Request:", "Status", sw.Status, "Duration", fmt.Sprintf("%vms", time.Since(start).Milliseconds()), "url", r.URL.Path)
	})
}

var CompressHandler func(http.Handler) http.Handler

func CompressFunc(f func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return CompressHandler(http.HandlerFunc(f))
}
func init() {
	CompressHandler, _ = httpcompression.DefaultAdapter()
}

func SSEFunc(f func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return SSEHandler(http.HandlerFunc(f))
}

func SSEHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			http.Error(w, "SSE only", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		next.ServeHTTP(w, r)
	})
}
