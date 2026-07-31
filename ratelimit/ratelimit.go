// Package ratelimit implements a Redis-backed, per-client, per-route-tier request rate
// limiter. Counters live in Redis (INCR + EXPIRE) rather than per-pod memory, so the limit is
// consistent across catalog-api's replicas instead of being N times looser depending on how
// many pods happen to be up.
package ratelimit

import (
	"context"
	"fmt"

	"github.com/gomodule/redigo/redis"
)

// Tier describes a rate-limit budget: at most Limit requests per Window.
type Tier struct {
	Limit  int
	Window int // seconds
}

// Limiter enforces per-client request limits per tier, backed by a Redis connection pool.
type Limiter struct {
	pool  *redis.Pool
	tiers map[string]Tier
}

// New builds a Limiter using the given Redis pool and tier definitions. Callers should pass at
// least a "standard" and "cheap" tier; Allow falls back to "standard" for an unknown tier name.
func New(pool *redis.Pool, tiers map[string]Tier) *Limiter {
	return &Limiter{pool: pool, tiers: tiers}
}

// Allow increments the request counter for clientKey within the named tier and reports whether
// the request is within that tier's limit. A non-nil error means the Redis backend was
// unreachable - callers must treat that as fail-closed (reject the request), not fail-open.
func (l *Limiter) Allow(ctx context.Context, clientKey, tierName string) (bool, error) {
	tier, ok := l.tiers[tierName]
	if !ok {
		tier = l.tiers["standard"]
	}

	conn, err := l.pool.GetContext(ctx)
	if err != nil {
		return false, fmt.Errorf("ratelimit: get redis connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	key := fmt.Sprintf("ratelimit:%s:%s", tierName, clientKey)
	count, err := redis.Int(conn.Do("INCR", key))
	if err != nil {
		return false, fmt.Errorf("ratelimit: incr: %w", err)
	}
	if count == 1 {
		if _, err := conn.Do("EXPIRE", key, tier.Window); err != nil {
			return false, fmt.Errorf("ratelimit: expire: %w", err)
		}
	}

	return count <= tier.Limit, nil
}

// Ping verifies the Redis backend is reachable, for use in startup/readiness checks.
func Ping(ctx context.Context, pool *redis.Pool) error {
	conn, err := pool.GetContext(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	_, err = conn.Do("PING")
	return err
}
