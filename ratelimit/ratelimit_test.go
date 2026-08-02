package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
)

func newTestPool(t *testing.T) (*redis.Pool, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", mr.Addr())
		},
	}
	t.Cleanup(func() { _ = pool.Close() })

	return pool, mr
}

func TestAllowWithinLimit(t *testing.T) {
	pool, _ := newTestPool(t)
	limiter := New(pool, map[string]Tier{"standard": {Limit: 3, Window: 60}})

	for i := 0; i < 3; i++ {
		allowed, err := limiter.Allow(context.Background(), "client-a", "standard")
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !allowed {
			t.Fatalf("Allow() request %d = false, want true (within limit)", i+1)
		}
	}
}

func TestAllowRejectsOverLimit(t *testing.T) {
	pool, _ := newTestPool(t)
	limiter := New(pool, map[string]Tier{"standard": {Limit: 2, Window: 60}})

	for i := 0; i < 2; i++ {
		if allowed, err := limiter.Allow(context.Background(), "client-a", "standard"); err != nil || !allowed {
			t.Fatalf("Allow() request %d = (%v, %v), want (true, nil)", i+1, allowed, err)
		}
	}

	allowed, err := limiter.Allow(context.Background(), "client-a", "standard")
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if allowed {
		t.Fatal("Allow() over limit = true, want false")
	}
}

func TestAllowTracksClientsIndependently(t *testing.T) {
	pool, _ := newTestPool(t)
	limiter := New(pool, map[string]Tier{"standard": {Limit: 1, Window: 60}})

	if allowed, err := limiter.Allow(context.Background(), "client-a", "standard"); err != nil || !allowed {
		t.Fatalf("Allow(client-a) = (%v, %v), want (true, nil)", allowed, err)
	}
	if allowed, err := limiter.Allow(context.Background(), "client-b", "standard"); err != nil || !allowed {
		t.Fatalf("Allow(client-b) = (%v, %v), want (true, nil) - separate client budget", allowed, err)
	}
	if allowed, _ := limiter.Allow(context.Background(), "client-a", "standard"); allowed {
		t.Fatal("Allow(client-a) second request = true, want false - client-a is over its own limit")
	}
}

func TestAllowTracksTiersIndependently(t *testing.T) {
	pool, _ := newTestPool(t)
	limiter := New(pool, map[string]Tier{
		"cheap":    {Limit: 5, Window: 60},
		"standard": {Limit: 1, Window: 60},
	})

	if allowed, err := limiter.Allow(context.Background(), "client-a", "standard"); err != nil || !allowed {
		t.Fatalf("Allow(standard) = (%v, %v), want (true, nil)", allowed, err)
	}
	if allowed, _ := limiter.Allow(context.Background(), "client-a", "standard"); allowed {
		t.Fatal("Allow(standard) second request = true, want false")
	}
	if allowed, err := limiter.Allow(context.Background(), "client-a", "cheap"); err != nil || !allowed {
		t.Fatalf("Allow(cheap) = (%v, %v), want (true, nil) - cheap tier has its own budget", allowed, err)
	}
}

func TestAllowUnknownTierFallsBackToStandard(t *testing.T) {
	pool, _ := newTestPool(t)
	limiter := New(pool, map[string]Tier{"standard": {Limit: 1, Window: 60}})

	if allowed, err := limiter.Allow(context.Background(), "client-a", "unknown-tier"); err != nil || !allowed {
		t.Fatalf("Allow(unknown-tier) = (%v, %v), want (true, nil) via standard fallback", allowed, err)
	}
	if allowed, _ := limiter.Allow(context.Background(), "client-a", "unknown-tier"); allowed {
		t.Fatal("Allow(unknown-tier) second request = true, want false - fell back to standard's limit of 1")
	}
}

func TestAllowFailsClosedWhenBackendUnreachable(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()

	// A fresh pool pointed at the now-closed address, rather than reusing a pool with an
	// already-established (now half-open) connection - otherwise the OS may not surface the
	// closed remote end until a long TCP timeout instead of an immediate connection refusal.
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", addr, redis.DialConnectTimeout(2*time.Second))
		},
	}
	t.Cleanup(func() { _ = pool.Close() })

	limiter := New(pool, map[string]Tier{"standard": {Limit: 10, Window: 60}})

	allowed, err := limiter.Allow(context.Background(), "client-a", "standard")
	if err == nil {
		t.Fatal("Allow() with unreachable backend error = nil, want non-nil")
	}
	if allowed {
		t.Fatal("Allow() with unreachable backend = true, want false (fail closed)")
	}
}

func TestPingSucceedsWhenReachable(t *testing.T) {
	pool, _ := newTestPool(t)

	if err := Ping(context.Background(), pool); err != nil {
		t.Fatalf("Ping() error = %v, want nil", err)
	}
}

func TestPingFailsWhenUnreachable(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()

	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", addr, redis.DialConnectTimeout(2*time.Second))
		},
	}
	t.Cleanup(func() { _ = pool.Close() })

	if err := Ping(context.Background(), pool); err == nil {
		t.Fatal("Ping() with unreachable backend error = nil, want non-nil")
	}
}
