package config

import (
	"flag"
	"log/slog"
	"os"
	"time"
)

// Config holds the application configuration
type Config struct {
	// ListenAddr is the address where the server will listen for requests
	ListenAddr string
	// UpstreamURL is the Prometheus server URL to forward requests to
	UpstreamURL string
	// CacheTTL is the time-to-live for cached query results
	CacheTTL time.Duration
	// LogLevel controls the logging verbosity
	LogLevel slog.Level
}

// Parse parses configuration from command-line flags and environment variables
func Parse() *Config {
	cfg := &Config{}

	// Command-line flags
	flag.StringVar(&cfg.ListenAddr, "listen", ":9091", "Address to listen on")
	flag.StringVar(&cfg.UpstreamURL, "upstream", "http://localhost:9090", "Prometheus upstream URL")
	flag.DurationVar(&cfg.CacheTTL, "ttl", 5*time.Minute, "Cache TTL duration")

	var logLevelStr string
	flag.StringVar(&logLevelStr, "log-level", "info", "Log level (debug, info, warn, error)")

	flag.Parse()

	// Environment variables override flags
	if addr := os.Getenv("PROMCACHE_LISTEN_ADDR"); addr != "" {
		cfg.ListenAddr = addr
	}
	if url := os.Getenv("PROMCACHE_UPSTREAM_URL"); url != "" {
		cfg.UpstreamURL = url
	}
	if ttl := os.Getenv("PROMCACHE_TTL"); ttl != "" {
		if parsed, err := time.ParseDuration(ttl); err == nil {
			cfg.CacheTTL = parsed
		}
	}
	if level := os.Getenv("PROMCACHE_LOG_LEVEL"); level != "" {
		logLevelStr = level
	}

	// Parse log level
	switch logLevelStr {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "info":
		cfg.LogLevel = slog.LevelInfo
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		cfg.LogLevel = slog.LevelInfo
	}

	return cfg
}
