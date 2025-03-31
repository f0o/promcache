# PromCache

PromCache is a caching proxy for Prometheus that reduces load on your Prometheus servers by caching query results. It sits between clients and Prometheus, serving cached responses when available and proxying requests to the upstream Prometheus server when needed.

While PromCache could be used to cache any HTTP API, it is specifically designed for Prometheus API endpoints. It intelligently normalizes query parameters to improve cache hit rates and provides configurable TTL-based caching.

PromCache works best with GET requests, as it caches the full response based on the URL and query parameters. It is not designed for caching POST requests or other HTTP methods.

## Features

- Transparent HTTP proxy for Prometheus API endpoints
- Configurable TTL-based caching for query results
- Cache hit/miss metrics
- Intelligent query parameter normalization for better cache hit rates
- Docker support with health checks
- Low memory footprint

## Installation

### Using Go

```bash
go install github.com/f0o/promcache/cmd/promcached@latest
```

### Using Docker

```bash
docker build -t promcache .
docker run -p 9091:9091 -e PROMCACHE_UPSTREAM_URL=http://prometheus:9090 promcache
```

## Configuration

PromCache can be configured using command-line flags or environment variables:

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `-listen` | `PROMCACHE_LISTEN_ADDR` | `:9091` | Address to listen on |
| `-upstream` | `PROMCACHE_UPSTREAM_URL` | `http://localhost:9090` | Prometheus upstream URL |
| `-ttl` | `PROMCACHE_TTL` | `5m` | Cache TTL duration |
| `-log-level` | `PROMCACHE_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

## API Endpoints

- `/api/*` - Proxied Prometheus API endpoints with caching
- `/metrics` - Prometheus metrics about the cache performance
- `/health` - Health check endpoint
- `/debug/cache` - Cache inspection endpoint (for debugging)

## Metrics

The following metrics are exposed at the `/metrics` endpoint:

- `promcache_cache_hits_total` - Total number of cache hits
- `promcache_cache_misses_total` - Total number of cache misses
- `promcache_upstream_request_duration_seconds` - Histogram of upstream request latencies
- `promcache_cache_size` - Current number of items in the cache

## Development

### Prerequisites

- Go 1.23 or later
- Docker (optional)

### Building from source

```bash
git clone https://github.com/f0o/promcache.git
cd promcache
go build -o promcached ./cmd/promcached
```

### Running tests

```bash
go test ./...
```

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0) - see the [LICENSE](LICENSE) file for details.