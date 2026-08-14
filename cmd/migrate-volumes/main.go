// Command migrate-volumes is the one-time cutover for catalog-entity-versioning's volume
// meta+version data model (see openspec's design.md Migration Plan). It backfills every
// existing "volumes" document into a meta record + a single live version, then converts every
// still-pending proposed_changes entry for a volume into a submitted version so no in-flight
// submitter proposal is silently dropped at cutover.
//
// Safe to re-run: MigrateVolumes skips any record that already has a meta record, and each
// proposal is only converted once per run (re-running after a partial failure will create
// duplicate submitted versions for proposals already converted in an earlier run - this command
// doesn't yet mark a proposal as migrated, since proposed_changes is superseded entirely once
// every entity type's migration completes, see tasks.md task group 8).
//
// Known limitation: a pending proposal's staged cover/sample assets (StagedCoverAssetId/
// StagedSampleAssetIds) aren't representable on a VolumeVersion yet - see tasks.md 3.7. Any
// text fields (title/description/notes) on such a proposal are still migrated; the staged asset
// reference itself is logged as skipped and needs manual follow-up.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/sweetrpg/catalog-api/proposedchanges"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/database"
)

func main() {
	_ = godotenv.Load(".env")
	logging.Init()
	database.SetupDatabase()
	defer database.TeardownDatabase()

	ctx := context.Background()

	migratedRecords, err := data.MigrateVolumes(ctx)
	if err != nil {
		logging.Logger.Error("migrate-volumes: backfill meta+version records failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("migrated %d volume record(s) into meta+version\n", migratedRecords)

	migratedProposals, skippedStagedAssets, err := migratePendingProposals(ctx)
	if err != nil {
		logging.Logger.Error("migrate-volumes: backfill pending proposals failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("migrated %d pending proposal(s) into submitted versions\n", migratedProposals)
	if skippedStagedAssets > 0 {
		fmt.Printf(
			"WARNING: %d proposal(s) referenced staged cover/sample assets that were NOT migrated - "+
				"needs manual follow-up, see this command's doc comment\n", skippedStagedAssets)
	}
}

// migratePendingProposals converts every still-pending "volume" proposed_changes entry into a
// submitted version, applying the proposal's diffed string fields onto the live record's
// current snapshot as the submitted version's base.
func migratePendingProposals(ctx context.Context) (migrated, skippedStagedAssets int, err error) {
	pending, err := proposedchanges.ListPendingByType(ctx, "volume")
	if err != nil {
		return 0, 0, err
	}

	for _, p := range pending {
		existing, err := data.GetVolume(ctx, p.RecordID)
		if err != nil {
			return migrated, skippedStagedAssets, err
		}
		if existing == nil {
			logging.Logger.Warn(
				"migrate-volumes: skipping proposal for a volume that no longer exists",
				"proposalId", p.ID.Hex(), "volumeId", p.RecordID)
			continue
		}

		updated := *existing
		for field, change := range p.Diff {
			newValue, ok := change.New.(string)
			if !ok {
				continue
			}
			switch field {
			case "title":
				updated.Title = newValue
			case "description":
				updated.Description = newValue
			case "notes":
				updated.Notes = newValue
			}
		}

		if _, err := data.CreateSubmittedVolumeVersion(
			ctx, p.RecordID, &updated, p.SubmittedBy, p.SubmittedAt); err != nil {
			return migrated, skippedStagedAssets, err
		}
		migrated++

		if p.StagedCoverAssetId != "" || len(p.StagedSampleAssetIds) > 0 {
			logging.Logger.Warn(
				"migrate-volumes: proposal referenced staged assets - not migrated onto the submitted version",
				"proposalId", p.ID.Hex(), "volumeId", p.RecordID)
			skippedStagedAssets++
		}
	}

	return migrated, skippedStagedAssets, nil
}
