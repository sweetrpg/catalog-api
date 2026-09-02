// Command backfill-volume-system-titles populates the denormalized system_titles map on every
// existing volume by resolving each referenced game system's current title from
// game-systems-api once. Idempotent and safe to re-run (UpdateVolumeSystemTitleBySystem is a
// no-op when the stored title is unchanged). Run once after deploying the sync consumer.
package main

import (
	"context"
	"sync"

	"github.com/joho/godotenv"
	apiutil "github.com/sweetrpg/api-core.go/util"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-data.go/gamesystems"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
	"github.com/sweetrpg/mongodb.go/database"
)

const backfillConcurrency = 8

func main() {
	_ = godotenv.Load(".env")
	logging.Init()

	database.SetupDatabase()
	defer database.TeardownDatabase()

	data.GameSystemsClient = gamesystems.NewClient(util.GetEnv(constants.GAME_SYSTEMS_API_URL, ""))

	ctx := context.Background()

	volumes, err := data.QueryVolumes(ctx, apiutil.QueryParams{Limit: 100000})
	if err != nil {
		logging.Logger.Error("backfill: list volumes failed", "error", err.Error())
		return
	}

	// One title lookup per distinct referenced system; UpdateVolumeSystemTitleBySystem then
	// fans that title out to every volume that references it.
	distinct := map[string]struct{}{}
	for _, v := range volumes {
		for _, s := range v.Systems {
			if s != nil && s.ID != "" {
				distinct[s.ID] = struct{}{}
			}
		}
	}
	logging.Logger.Info("backfill: starting", "volumes", len(volumes), "distinct_systems", len(distinct))

	var wg sync.WaitGroup
	sem := make(chan struct{}, backfillConcurrency)
	var mu sync.Mutex
	var updated, skipped int

	for id := range distinct {
		wg.Add(1)
		go func(systemID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sys, err := data.GetSystem(ctx, systemID)
			if err != nil || sys == nil || sys.GameSystem == "" {
				logging.Logger.Warn("backfill: system unresolved, left unset", "system_id", systemID, "error", err)
				mu.Lock()
				skipped++
				mu.Unlock()
				return
			}
			affected, err := data.UpdateVolumeSystemTitleBySystem(ctx, systemID, sys.GameSystem)
			if err != nil {
				logging.Logger.Error("backfill: update failed", "system_id", systemID, "error", err.Error())
				return
			}
			mu.Lock()
			updated += len(affected)
			mu.Unlock()
		}(id)
	}
	wg.Wait()

	logging.Logger.Info("backfill: done", "volume_versions_updated", updated, "systems_skipped", skipped)
}
