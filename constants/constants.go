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

	// AUTH_API_URL points at auth-api's base URL (e.g.
	// http://api-v1.sweetrpg-auth.svc.cluster.local:8000), used to verify bearer tokens and
	// resolve roles via POST /authz/check for write endpoints. See platform docs/openspec.md's
	// volume-edit-with-approval-workflow change.
	AUTH_API_URL = "AUTH_API_URL"

	// ASSETS_WEB_URL points at assets-web's base URL (e.g.
	// http://api-v1.sweetrpg-assets.svc.cluster.local:8000), used to promote a staged
	// cover/sample asset to live (or reclaim it) on volume edit session finalize/accept/reject.
	// See durable-volume-editing in sweetrpg/platform.
	ASSETS_WEB_URL = "ASSETS_WEB_URL"

	// GAMESYSTEMS_API_URL points at gamesystems-api's base URL (e.g.
	// http://api-v1.sweetrpg-gamesystems.svc.cluster.local:8000), the system of record a
	// volume's system references resolve against. See game-systems-service in sweetrpg/platform.
	GAMESYSTEMS_API_URL = "GAMESYSTEMS_API_URL"
)

// Value constants
const (
	ServiceName = "catalog-api"

	// ProfilingEnabledFlag is the feature-flag key gating continuous
	// profiling, evaluated via api-core.go/featureflags. Replaces the old
	// PYROSCOPE_SERVER_ADDRESS-presence check; see
	// openspec/changes/pyroscope-profiling-feature-flag in sweetrpg/platform.
	ProfilingEnabledFlag = "profiling-enabled"

	ErrorCacheUnavailable     = "cache_unavailable"
	ErrorRateLimitUnavailable = "rate_limit_unavailable"
)
