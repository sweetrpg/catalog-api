// Package readiness tracks the reachability of backend dependencies (beyond Mongo, which
// api-core.go's HealthHandler already covers) so /status/health can fail loud instead of the
// service silently degrading to an uncached or unlimited mode.
package readiness

import "sync/atomic"

var cacheReady atomic.Bool

// SetCacheReady records whether the configured cache backend (Redis, when REDIS_HOST is set)
// is currently reachable. Services without REDIS_HOST configured should call this with true,
// since the in-memory fallback store has no external dependency to fail.
func SetCacheReady(ready bool) {
	cacheReady.Store(ready)
}

// CacheReady reports the last-known reachability of the configured cache backend.
func CacheReady() bool {
	return cacheReady.Load()
}
