// Package readiness tracks the reachability of backend dependencies (beyond Mongo, which
// api-core.go's HealthHandler already covers) so /status/health can fail loud instead of the
// service silently degrading to an uncached or unlimited mode.
package readiness

import (
	"context"
	"sync/atomic"

	"github.com/gomodule/redigo/redis"
	"github.com/sweetrpg/catalog-api/ratelimit"
)

var cachePool atomic.Pointer[redis.Pool]

// SetCachePool records the Redis pool CacheReady live-pings on each call. Pass nil when no
// cache backend is configured (REDIS_HOST unset) - CacheReady then reports true, since the
// in-memory fallback store has no external dependency to fail.
func SetCachePool(pool *redis.Pool) {
	cachePool.Store(pool)
}

// CacheReady live-pings the configured cache backend rather than returning a cached boot-time
// result - a Redis outage that resolves on its own should let readiness recover without
// requiring a pod restart.
func CacheReady(ctx context.Context) bool {
	pool := cachePool.Load()
	if pool == nil {
		return true
	}
	return ratelimit.Ping(ctx, pool) == nil
}
