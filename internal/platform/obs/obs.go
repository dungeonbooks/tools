// Package obs provides shared logging, a health endpoint, and HTTP middleware.
package obs

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

func NewLogger(env string) *slog.Logger {
	if env == "dev" {
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Logging logs each request and recovers panics.
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &recorder{ResponseWriter: w, status: http.StatusOK}
			defer func() {
				if rv := recover(); rv != nil {
					http.Error(w, "internal error", http.StatusInternalServerError)
					log.Error("panic", "err", rv, "path", r.URL.Path)
					rec.status = http.StatusInternalServerError
				}
				log.Info("request", "method", r.Method, "path", r.URL.Path,
					"status", rec.status, "dur_ms", time.Since(start).Milliseconds())
			}()
			next.ServeHTTP(rec, r)
		})
	}
}
