// Package proxy provides HTTP caching proxy functionality
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/f0o/promcache/internal/cache"
)

// Headers that shouldn't be cached
var skipCacheHeaders = []string{
	"Date",
	"Connection",
	"Transfer-Encoding",
	"Keep-Alive",
}

// Response represents a cached HTTP response
type Response struct {
	Headers    http.Header `json:"headers"`
	StatusCode int         `json:"status_code"`
	Body       []byte      `json:"body"`
}

// HTTPCacheProxy forwards requests to an upstream server and caches the responses
type HTTPCacheProxy struct {
	upstreamURL string
	cache       *cache.Cache
	client      *http.Client
	log         *slog.Logger
	cacheTTL    time.Duration
}

// New creates a new HTTP caching proxy
func New(upstreamURL string, cache *cache.Cache, log *slog.Logger) *HTTPCacheProxy {
	return &HTTPCacheProxy{
		upstreamURL: upstreamURL,
		cache:       cache,
		client: &http.Client{
			Timeout: 30 * time.Second, // Add reasonable timeout
		},
		log:      log,
		cacheTTL: cache.TTL(),
	}
}

// HandleRequest processes an incoming request, checking the cache first
// and forwarding to the upstream if necessary
func (p *HTTPCacheProxy) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// Only cache GET requests
	isCacheable := r.Method == http.MethodGet

	// Generate cache key from request
	cacheKey := p.generateCacheKey(r)
	p.log.Debug("Request received",
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
		"key", cacheKey,
		"cacheable", isCacheable)

	// Try to get from cache for cacheable requests
	if isCacheable && p.tryServeCachedResponse(w, r, cacheKey) {
		return
	}

	// Cache miss or non-cacheable request, forward to upstream
	p.log.Info("Cache miss, forwarding to upstream",
		"path", r.URL.Path,
		"key", cacheKey)
	p.forwardRequest(w, r, cacheKey, isCacheable)
}

// tryServeCachedResponse attempts to serve a response from cache
// Returns true if successful, false otherwise
func (p *HTTPCacheProxy) tryServeCachedResponse(w http.ResponseWriter, r *http.Request, cacheKey string) bool {
	data, found := p.cache.Get(cacheKey)
	if !found {
		return false
	}

	p.log.Info("Serving from cache",
		"path", r.URL.Path,
		"key", cacheKey)

	var cachedResp Response
	if err := json.Unmarshal(data, &cachedResp); err != nil {
		p.log.Error("Failed to unmarshal cached response",
			"error", err,
			"key", cacheKey)
		return false
	}

	// Write headers from cache
	for name, values := range cachedResp.Headers {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.Header().Set("X-Cache", "HIT")

	// Send response
	w.WriteHeader(cachedResp.StatusCode)
	w.Write(cachedResp.Body)
	return true
}

// forwardRequest forwards a request to the upstream server
func (p *HTTPCacheProxy) forwardRequest(w http.ResponseWriter, r *http.Request, cacheKey string, isCacheable bool) {
	// Prepare upstream request
	upstreamReq, err := p.prepareUpstreamRequest(r)
	if err != nil {
		p.log.Error("Failed to prepare upstream request",
			"error", err,
			"path", r.URL.Path)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Send request to upstream
	startTime := time.Now()
	resp, err := p.client.Do(upstreamReq)
	requestDuration := time.Since(startTime)

	if err != nil {
		p.log.Error("Failed to forward request to upstream",
			"error", err,
			"duration_ms", requestDuration.Milliseconds(),
			"path", r.URL.Path)
		http.Error(w, "Failed to reach upstream server", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		p.log.Error("Failed to read upstream response",
			"error", err,
			"path", r.URL.Path)
		http.Error(w, "Failed to read upstream response", http.StatusInternalServerError)
		return
	}

	p.log.Debug("Received upstream response",
		"status", resp.StatusCode,
		"size", len(respBody),
		"duration_ms", requestDuration.Milliseconds(),
		"path", r.URL.Path)

	// Cache successful responses
	if isCacheable && resp.StatusCode == http.StatusOK {
		p.cacheResponse(cacheKey, resp, respBody)
	}

	// Send response to client
	p.writeResponse(w, resp, respBody)
}

// prepareUpstreamRequest creates a new request to the upstream server
func (p *HTTPCacheProxy) prepareUpstreamRequest(r *http.Request) (*http.Request, error) {
	// Parse upstream URL
	upstream, err := url.Parse(p.upstreamURL)
	if err != nil {
		return nil, err
	}

	// Construct full URL
	upstream.Path = r.URL.Path
	upstream.RawQuery = r.URL.RawQuery

	// Read and preserve request body
	var bodyReader io.Reader
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}

		// Restore original request body and create a new reader for upstream
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	// Create upstream request
	upstreamReq, err := http.NewRequestWithContext(
		r.Context(),
		r.Method,
		upstream.String(),
		bodyReader,
	)
	if err != nil {
		return nil, err
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			upstreamReq.Header.Add(name, value)
		}
	}

	return upstreamReq, nil
}

// cacheResponse stores a successful response in the cache
func (p *HTTPCacheProxy) cacheResponse(cacheKey string, resp *http.Response, body []byte) {
	// Create cached response object
	cachedResp := Response{
		Headers:    make(http.Header),
		StatusCode: resp.StatusCode,
		Body:       body,
	}

	// Copy headers except those that shouldn't be cached
	for name, values := range resp.Header {
		shouldSkip := false
		for _, skipHeader := range skipCacheHeaders {
			if strings.EqualFold(name, skipHeader) {
				shouldSkip = true
				break
			}
		}

		if !shouldSkip {
			cachedResp.Headers[name] = values
		}
	}

	// Serialize and store in cache
	cachedData, err := json.Marshal(cachedResp)
	if err != nil {
		p.log.Error("Failed to marshal response for caching",
			"error", err,
			"key", cacheKey)
		return
	}

	p.log.Debug("Caching response",
		"key", cacheKey,
		"status", resp.StatusCode,
		"size", len(body))
	p.cache.Set(cacheKey, cachedData)
}

// writeResponse sends the response to the client
func (p *HTTPCacheProxy) writeResponse(w http.ResponseWriter, resp *http.Response, body []byte) {
	// Copy headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.Header().Set("X-Cache", "MISS")

	// Send response
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// generateCacheKey creates a unique key for caching based on the request
func (p *HTTPCacheProxy) generateCacheKey(r *http.Request) string {
	// Copy query parameters to avoid modifying the original
	query := make(url.Values, len(r.URL.Query()))
	for k, v := range r.URL.Query() {
		query[k] = append([]string{}, v...)
	}

	// Round time parameters for better cache hit rate
	ttlSeconds := int64(p.cacheTTL.Seconds())
	if ttlSeconds > 0 {
		p.roundTimeParameter(query, "time", ttlSeconds, false)
		p.roundTimeParameter(query, "start", ttlSeconds, false)
		p.roundTimeParameter(query, "end", ttlSeconds, true)
	}

	// Build final key
	return r.Method + ":" + r.URL.Path + ":" + p.normalizeQueryString(query)
}

// normalizeQueryString creates a consistent string from URL query parameters
func (p *HTTPCacheProxy) normalizeQueryString(query url.Values) string {
	if len(query) == 0 {
		return ""
	}

	// Get sorted keys for consistent ordering
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build the normalized query string
	var b strings.Builder
	b.Grow(128) // Pre-allocate buffer for better performance

	for i, k := range keys {
		values := query[k]
		sort.Strings(values) // Sort values for consistency

		for j, v := range values {
			if i > 0 || j > 0 {
				b.WriteByte('&')
			}
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(v)
		}
	}

	return b.String()
}

// roundTimeParameter rounds a time parameter to the nearest TTL boundary
func (p *HTTPCacheProxy) roundTimeParameter(query url.Values, paramName string, ttlSeconds int64, roundUp bool) {
	if paramStr := query.Get(paramName); paramStr != "" {
		paramTime, err := strconv.ParseFloat(paramStr, 64)
		if err != nil {
			return // Skip if not a valid number
		}

		var roundedTime int64
		if roundUp {
			// Round up to next TTL boundary
			roundedTime = ((int64(paramTime) + ttlSeconds - 1) / ttlSeconds) * ttlSeconds
		} else {
			// Round down to previous TTL boundary
			roundedTime = (int64(paramTime) / ttlSeconds) * ttlSeconds
		}

		query.Set(paramName, strconv.FormatInt(roundedTime, 10))
	}
}
