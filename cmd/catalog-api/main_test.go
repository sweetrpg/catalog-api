package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	apiconstants "github.com/sweetrpg/api-core.go/constants"
	"github.com/sweetrpg/catalog-api/readiness"
	"github.com/sweetrpg/common.go/logging"
)

func TestMain(m *testing.M) {
	logging.Init()
	os.Exit(m.Run())
}

func TestSetupCacheReportsReadyWhenRedisReachable(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Setenv(apiconstants.REDIS_HOST, mr.Host())
	t.Setenv(apiconstants.REDIS_PORT, mr.Port())

	pool := setupRedisPool()
	t.Cleanup(func() { _ = pool.Close() })

	setupCache(pool)

	if !readiness.CacheReady(context.Background()) {
		t.Fatal("readiness.CacheReady() = false, want true when Redis is reachable")
	}
}

func TestSetupCacheFailsReadinessWhenRedisUnreachable(t *testing.T) {
	mr := miniredis.RunT(t)
	host, port := mr.Host(), mr.Port()
	mr.Close()

	t.Setenv(apiconstants.REDIS_HOST, host)
	t.Setenv(apiconstants.REDIS_PORT, port)

	pool := setupRedisPool()
	t.Cleanup(func() { _ = pool.Close() })

	setupCache(pool)

	if readiness.CacheReady(context.Background()) {
		t.Fatal("readiness.CacheReady() = true, want false when REDIS_HOST is configured but unreachable")
	}
}

func TestSetupCacheReportsReadyWhenRedisNotConfigured(t *testing.T) {
	// Ensure no leftover REDIS_HOST from another test/process poisons this case.
	previous, wasSet := os.LookupEnv(apiconstants.REDIS_HOST)
	_ = os.Unsetenv(apiconstants.REDIS_HOST)
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(apiconstants.REDIS_HOST, previous)
		}
	})

	setupCache(nil)

	if !readiness.CacheReady(context.Background()) {
		t.Fatal("readiness.CacheReady() = false, want true when no cache backend is configured (in-memory store, no dependency to fail)")
	}
}

func TestCacheInvalidationMiddlewareFlushesOnSuccessfulWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := persistence.NewInMemoryStore(time.Minute)
	if err := store.Set("some-cached-get", "stale value", persistence.DEFAULT); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}

	r := gin.New()
	r.Use(cacheInvalidationMiddleware(store))
	r.PATCH("/publishers/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPatch, "/publishers/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var got string
	if err := store.Get("some-cached-get", &got); err != persistence.ErrCacheMiss {
		t.Fatalf("cache entry after a successful write: err = %v, want ErrCacheMiss (flushed)", err)
	}
}

func TestCacheInvalidationMiddlewareLeavesCacheOnFailedWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := persistence.NewInMemoryStore(time.Minute)
	if err := store.Set("some-cached-get", "still fresh", persistence.DEFAULT); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}

	r := gin.New()
	r.Use(cacheInvalidationMiddleware(store))
	r.PATCH("/publishers/:id", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	req := httptest.NewRequest(http.MethodPatch, "/publishers/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var got string
	if err := store.Get("some-cached-get", &got); err != nil || got != "still fresh" {
		t.Fatalf("cache entry after a failed write: got = %q, err = %v, want unchanged", got, err)
	}
}

func TestCacheInvalidationMiddlewareLeavesCacheOnRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := persistence.NewInMemoryStore(time.Minute)
	if err := store.Set("some-cached-get", "still fresh", persistence.DEFAULT); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}

	r := gin.New()
	r.Use(cacheInvalidationMiddleware(store))
	r.GET("/publishers/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/publishers/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var got string
	if err := store.Get("some-cached-get", &got); err != nil || got != "still fresh" {
		t.Fatalf("cache entry after a GET: got = %q, err = %v, want unchanged", got, err)
	}
}
