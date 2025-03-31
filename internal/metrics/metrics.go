package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	cacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promcache_cache_hits_total",
		Help: "The total number of cache hits",
	})

	cacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "promcache_cache_misses_total",
		Help: "The total number of cache misses",
	})

	upstreamLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "promcache_upstream_request_duration_seconds",
		Help:    "Upstream request latency in seconds",
		Buckets: prometheus.DefBuckets,
	})

	cacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "promcache_cache_size",
		Help: "Current number of items in the cache",
	})
)

// RecordCacheHit increments the cache hit counter
func RecordCacheHit() {
	cacheHits.Inc()
}

// RecordCacheMiss increments the cache miss counter
func RecordCacheMiss() {
	cacheMisses.Inc()
}

// RecordUpstreamLatency records the latency of an upstream request
func RecordUpstreamLatency(seconds float64) {
	upstreamLatency.Observe(seconds)
}

// SetCacheSize updates the cache size gauge
func SetCacheSize(size float64) {
	cacheSize.Set(size)
}

// Handler returns an HTTP handler for metrics
func Handler() http.Handler {
	return promhttp.Handler()
}
