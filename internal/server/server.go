package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/f0o/promcache/internal/cache"
	"github.com/f0o/promcache/internal/metrics"
	"github.com/f0o/promcache/pkg/proxy"
)

// Server represents the HTTP server for the Prometheus cache
type Server struct {
	server *http.Server
	log    *slog.Logger
}

// New creates a new HTTP server
func New(listenAddr string, upstreamURL string, cache *cache.Cache, log *slog.Logger) *Server {
	// Create proxy
	promProxy := proxy.New(upstreamURL, cache, log)

	// Create router
	mux := http.NewServeMux()

	// Prometheus API endpoints
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		promProxy.HandleRequest(w, r)
	})

	// Metrics endpoint
	mux.Handle("/metrics", metrics.Handler())

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Debug cache endpoint
	mux.HandleFunc("/debug/cache", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			keys := cache.Keys()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"num_keys": len(keys),
				"keys":     keys,
			})
		}
	})

	// Create server
	srv := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	return &Server{
		server: srv,
		log:    log,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.log.Info("Starting server", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down server")
	return s.server.Shutdown(ctx)
}
