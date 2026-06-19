package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dungeonbooks/tools/internal/discovery"
	"github.com/dungeonbooks/tools/internal/platform/config"
	"github.com/dungeonbooks/tools/internal/platform/obs"
)

func main() {
	cfg := config.Load()
	log := obs.NewLogger(cfg.Env)

	hc := &http.Client{Timeout: 30 * time.Second}
	svc := discovery.NewService(
		discovery.NewExaClient(cfg.ExaAPIKey, hc),
		discovery.NewISBNResolver(hc),
		log,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", obs.Health())
	mux.HandleFunc("POST /v1/discover", func(w http.ResponseWriter, r *http.Request) {
		if cfg.ExaAPIKey == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "EXA_API_KEY not set"})
			return
		}
		var req struct {
			Query      string `json:"query"`
			NumResults int    `json:"numResults"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Query == "" || len(req.Query) > 500 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid query is required"})
			return
		}
		if req.NumResults <= 0 || req.NumResults > 25 {
			req.NumResults = 15
		}
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		books, err := svc.Discover(ctx, req.Query, req.NumResults)
		if err != nil {
			log.Error("discover failed", "err", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "discovery failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"books": books})
	})

	serve(cfg.Port, obs.Logging(log)(mux), log)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func serve(port string, h http.Handler, log *slog.Logger) {
	srv := &http.Server{Addr: ":" + port, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
