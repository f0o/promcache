package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/f0o/promcache/internal/cache"
	"github.com/f0o/promcache/internal/config"
	"github.com/f0o/promcache/internal/server"
)

func main() {
	// Parse configuration
	cfg := config.Parse()

	// Setup logging
	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	logger.Info("Starting promcache",
		"listen", cfg.ListenAddr,
		"upstream", cfg.UpstreamURL,
		"ttl", cfg.CacheTTL,
	)

	// Create cache
	c := cache.New(cfg.CacheTTL, logger)

	// Create and start server
	srv := server.New(cfg.ListenAddr, cfg.UpstreamURL, c, logger)

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()
	logger.Info("Server started")

	// Wait for interrupt signal
	<-done
	logger.Info("Shutting down...")

	// Gracefully shutdown with a 5-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped")
}
