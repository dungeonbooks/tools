package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dungeonbooks/tools/internal/bookmeta"
	"github.com/dungeonbooks/tools/internal/candidates"
	"github.com/dungeonbooks/tools/internal/platform/config"
	"github.com/dungeonbooks/tools/internal/platform/obs"
)

func main() {
	cfg := config.Load()
	log := obs.NewLogger(cfg.Env)
	store := candidates.NewStore()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", obs.Health())

	mux.HandleFunc("GET /v1/candidates", func(w http.ResponseWriter, r *http.Request) {
		list := store.List(candidates.ListFilter{
			Status:       candidates.Status(r.URL.Query().Get("status")),
			IngramStatus: candidates.IngramStatus(r.URL.Query().Get("ingram_status")),
		})
		writeJSON(w, http.StatusOK, map[string]any{"candidates": list})
	})

	mux.HandleFunc("POST /v1/candidates", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Books []bookmeta.Book `json:"books"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Books) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "books is required"})
			return
		}
		saved := make([]candidates.Candidate, 0, len(req.Books))
		for _, b := range req.Books {
			saved = append(saved, store.Upsert(candidates.Candidate{
				ISBN13: b.ISBN13, Title: b.Title, Author: b.Author, Publisher: b.Publisher,
				WhyTrending: b.WhyTrending, SourceURL: b.SourceURL, CoverURL: b.CoverURL,
			}))
		}
		writeJSON(w, http.StatusOK, map[string]any{"candidates": saved})
	})

	mux.HandleFunc("POST /v1/candidates/{id}/dismiss", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad id"})
			return
		}
		if err := store.SetStatus(id, candidates.StatusDismissed); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
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
