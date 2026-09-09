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

	// AUTH_API_URL points at auth-api's base URL (e.g.
	// http://api-v1.sweetrpg-auth.svc.cluster.local:8000), used to verify bearer tokens and
	// resolve roles via POST /authz/check for write endpoints. See platform docs/openspec.md's
	// volume-edit-with-approval-workflow change.
	AUTH_API_URL = "AUTH_API_URL"

	// USERS_API_URL points at users-api's base URL (e.g.
	// http://api-v1.sweetrpg-users.svc.cluster.local:8000), used to resolve the verified
	// subject to its canonical users._id for write-path created_by/updated_by stamps. See
	// canonical-user-ids-across-services in sweetrpg/platform.
	USERS_API_URL = "USERS_API_URL"

	// ASSETS_WEB_URL points at assets-web's base URL (e.g.
	// http://api-v1.sweetrpg-assets.svc.cluster.local:8000), used to promote a staged
	// cover/sample asset to live (or reclaim it) on volume edit session finalize/accept/reject.
	// See durable-volume-editing in sweetrpg/platform.
	ASSETS_WEB_URL = "ASSETS_WEB_URL"

	// GAME_SYSTEMS_API_URL points at game-systems-api's base URL (e.g.
	// http://api-v1.sweetrpg-game-systems.svc.cluster.local:8000), the system of record a
	// volume's system references resolve against. See game-systems-service in sweetrpg/platform.
	GAME_SYSTEMS_API_URL = "GAME_SYSTEMS_API_URL"
)

// Value constants
const (
	ServiceName = "catalog-api"

	// ProfilingEnabledFlag is the feature-flag key gating continuous
	// profiling, evaluated via api-core.go/featureflags. Replaces the old
	// PYROSCOPE_SERVER_ADDRESS-presence check; see
	// openspec/changes/pyroscope-profiling-feature-flag in sweetrpg/platform.
	ProfilingEnabledFlag = "profiling-enabled"

	ErrorCacheUnavailable = "cache_unavailable"
)
