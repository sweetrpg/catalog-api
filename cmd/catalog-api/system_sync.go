package main

import (
	"context"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gomodule/redigo/redis"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/internal/events"
	"github.com/sweetrpg/catalog-api/internal/sysdeletes"
	"github.com/sweetrpg/catalog-api/server"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
)

// startSystemSync starts the background work that follows game-systems-api: title refresh and
// deleted-reference cleanup from events, plus reconciliation over HTTP, which keeps working
// while the broker is down. None of it delays or blocks serving. The returned function stops it.
func startSystemSync(cache persistence.CacheStore, redisPool *redis.Pool) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	cleanup := server.CleanupDeletedSystem(func() { flushResponseCache(cache, redisPool) })

	// Nil (disabled) when NATS_URL is unset; Start and Stop accept that.
	consumer, err := events.NewConsumer(ctx)
	if err != nil {
		logging.Logger.Error("Failed to initialize game system event consumers", "error", err.Error())
	}
	consumer.Start(server.SyncSystemTitle(cache))
	consumer.StartDeletes(cleanup)

	reconciler := sysdeletes.Reconciler{
		List:  sysdeletes.NewClient(util.GetEnv(constants.GAME_SYSTEMS_API_URL, "")).List,
		Known: server.RecordedSystemDeletions,
		Apply: cleanup,
	}
	go reconciler.Run(ctx, sysdeletes.ParseInterval(util.GetEnv(constants.DELETED_SYSTEM_RECONCILE_INTERVAL, "")))

	return func() {
		cancel()
		consumer.Stop()
	}
}
