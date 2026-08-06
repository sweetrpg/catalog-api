package main

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
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
