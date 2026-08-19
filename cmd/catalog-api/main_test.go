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
	"github.com/gomodule/redigo/redis"
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
	r.Use(cacheInvalidationMiddleware(store, nil))
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
	r.Use(cacheInvalidationMiddleware(store, nil))
	r.PATCH("/publishers/:id", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	req := httptest.NewRequest(http.MethodPatch, "/publishers/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var got string
	if err := store.Get("some-cached-get", &got); err != nil || got != "still fresh" {
		t.Fatalf("cache entry after a failed write: got = %q, err = %v, want unchanged", got, err)
	}
}

func TestCacheInvalidationMiddlewareFlushesOnlyCacheDBNotEditSessionDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)

	cacheConn, err := redis.Dial("tcp", mr.Addr())
	if err != nil {
		t.Fatalf("dialing cache DB: %v", err)
	}
	if _, err := cacheConn.Do("SET", "some-cached-get", "stale value"); err != nil {
		t.Fatalf("seeding cache DB: %v", err)
	}
	_ = cacheConn.Close()

	sessionConn, err := redis.Dial("tcp", mr.Addr(), redis.DialDatabase(editSessionRedisDB))
	if err != nil {
		t.Fatalf("dialing edit-session DB: %v", err)
	}
	if _, err := sessionConn.Do("SET", "edit-session:user:volume", "in-flight edit"); err != nil {
		t.Fatalf("seeding edit-session DB: %v", err)
	}
	_ = sessionConn.Close()

	redisPool := &redis.Pool{
		Dial: func() (redis.Conn, error) { return redis.Dial("tcp", mr.Addr()) },
	}
	defer func() { _ = redisPool.Close() }()

	store := persistence.NewInMemoryStore(time.Minute)
	r := gin.New()
	r.Use(cacheInvalidationMiddleware(store, redisPool))
	r.PATCH("/publishers/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPatch, "/publishers/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	cacheConn, err = redis.Dial("tcp", mr.Addr())
	if err != nil {
		t.Fatalf("dialing cache DB: %v", err)
	}
	defer func() { _ = cacheConn.Close() }()
	if exists, err := redis.Int(cacheConn.Do("EXISTS", "some-cached-get")); err != nil || exists != 0 {
		t.Fatalf("cache DB key after write: exists = %v, err = %v, want gone (flushed)", exists, err)
	}

	sessionConn, err = redis.Dial("tcp", mr.Addr(), redis.DialDatabase(editSessionRedisDB))
	if err != nil {
		t.Fatalf("dialing edit-session DB: %v", err)
	}
	defer func() { _ = sessionConn.Close() }()
	if got, err := redis.String(sessionConn.Do("GET", "edit-session:user:volume")); err != nil || got != "in-flight edit" {
		t.Fatalf("edit-session DB key after write: got = %q, err = %v, want untouched - a FLUSHALL "+
			"here would silently delete every in-flight volume edit session platform-wide", got, err)
	}
}

func TestCacheInvalidationMiddlewareLeavesCacheOnRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := persistence.NewInMemoryStore(time.Minute)
	if err := store.Set("some-cached-get", "still fresh", persistence.DEFAULT); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}

	r := gin.New()
	r.Use(cacheInvalidationMiddleware(store, nil))
	r.GET("/publishers/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/publishers/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var got string
	if err := store.Get("some-cached-get", &got); err != nil || got != "still fresh" {
		t.Fatalf("cache entry after a GET: got = %q, err = %v, want unchanged", got, err)
	}
}
