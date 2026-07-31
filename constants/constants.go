package constants

// Environment variable names
const (
	HEALTH_TOKEN             = "HEALTH_TOKEN"
	ALLOWED_ORIGINS          = "ALLOWED_ORIGINS"
	PYROSCOPE_SERVER_ADDRESS = "PYROSCOPE_SERVER_ADDRESS"
	PYROSCOPE_TENANT_ID      = "PYROSCOPE_TENANT_ID"

	// CACHE_TTLS maps route group to TTL, e.g. "licenses=30m,volumes=15m".
	// Route groups not listed fall back to CACHE_DEFAULT_TTL.
	CACHE_TTLS        = "CACHE_TTLS"
	CACHE_DEFAULT_TTL = "CACHE_DEFAULT_TTL"

	// DISTRIBUTED_RATE_LIMIT_ENABLED toggles the Redis-backed per-client limiter on in
	// place of the process-wide golang.org/x/time/rate limiter. Requires REDIS_HOST.
	DISTRIBUTED_RATE_LIMIT_ENABLED = "DISTRIBUTED_RATE_LIMIT_ENABLED"
	RATE_LIMIT_CHEAP               = "RATE_LIMIT_CHEAP"
	RATE_LIMIT_CHEAP_WINDOW        = "RATE_LIMIT_CHEAP_WINDOW_SECONDS"
	RATE_LIMIT_STANDARD            = "RATE_LIMIT_STANDARD"
	RATE_LIMIT_STANDARD_WINDOW     = "RATE_LIMIT_STANDARD_WINDOW_SECONDS"
)

// Value constants
const (
	ServiceName = "catalog-api"

	ErrorCacheUnavailable     = "cache_unavailable"
	ErrorRateLimitUnavailable = "rate_limit_unavailable"
)
