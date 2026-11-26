// Command consumer runs the game system JetStream consumers (title sync and reference cleanup) as a standalone process, for
// local development without the full catalog-api server. In production the same consumer runs
// as a background worker of cmd/catalog-api.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/sweetrpg/catalog-api/internal/events"
	"github.com/sweetrpg/catalog-api/server"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/database"
)

func main() {
	_ = godotenv.Load(".env")
	logging.Init()

	database.SetupDatabase()
	defer database.TeardownDatabase()

	consumer, err := events.NewConsumer(context.Background())
	if err != nil {
		logging.Logger.Error("consumer: init failed", "error", err.Error())
		os.Exit(1)
	}
	if consumer == nil {
		logging.Logger.Error("consumer: NATS_URL not set")
		os.Exit(1)
	}
	defer consumer.Stop()

	// No cache or publisher here - a standalone consumer only updates stored data; the
	// catalog-api server process owns cache invalidation and change events.
	consumer.Start(server.SyncSystemTitle(nil))
	consumer.StartDeletes(server.CleanupDeletedSystem(nil))
	logging.Logger.Info("consumer: running, press Ctrl-C to stop")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logging.Logger.Info("consumer: shutting down")
}
