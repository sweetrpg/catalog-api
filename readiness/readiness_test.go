package readiness

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
)

func TestCacheReadyWithNoPoolConfigured(t *testing.T) {
	SetCachePool(nil)
	if !CacheReady(context.Background()) {
		t.Fatal("CacheReady() = false with no pool configured, want true")
	}
}

func TestCacheReadyLivePingsRatherThanCaching(t *testing.T) {
	s := miniredis.RunT(t)
	addr := s.Addr()
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) { return redis.Dial("tcp", addr) },
	}
	SetCachePool(pool)
	defer SetCachePool(nil)

	if !CacheReady(context.Background()) {
		t.Fatal("CacheReady() = false with a reachable pool, want true")
	}

	s.Close()

	if CacheReady(context.Background()) {
		t.Fatal("CacheReady() = true after the backend closed, want false - check should be live, not cached")
	}
}
