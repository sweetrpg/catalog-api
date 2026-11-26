package server

import (
	"context"
	"fmt"

	"github.com/sweetrpg/catalog-api/internal/events"
	"github.com/sweetrpg/catalog-data.go/data"
	modelcore "github.com/sweetrpg/model-core.go/models"
)

// CleanupDeletedSystem records a game system deletion and removes catalog's references to it
// (PADR-0047). It serves both the delete consumer and the reconciler, and is idempotent.
// flushCache clears the response cache after a change; it may be nil. The whole cache is
// cleared, as after any write, because cached keys include query strings.
func CleanupDeletedSystem(flushCache func()) events.SystemDeleteHandler {
	return func(ctx context.Context, deletion events.SystemDeletion) error {
		system := data.DeletedSystem{ID: deletion.ID, SystemID: deletion.SystemID, SourceDeletedAt: deletion.DeletedAt}
		if err := data.RecordDeletedSystem(ctx, system, modelcore.SystemActor); err != nil {
			return fmt.Errorf("record deletion: %w", err)
		}
		// result holds what changed even when err is set, so announce it either way.
		result, err := data.RemoveDeletedSystemReferences(ctx, system, modelcore.SystemActor)
		if flushCache != nil && (len(result.Changed) > 0 || len(result.PendingRecordIDs) > 0) {
			flushCache()
		}
		for _, v := range result.Changed {
			volumeEventPublisher.PublishEntityUpdated(ctx, "volume", v.RecordID, v.Version, map[string]interface{}{
				"title": v.Title,
			})
		}
		return err
	}
}

// RecordedSystemDeletions returns the deletions catalog has tombstones for.
func RecordedSystemDeletions(ctx context.Context) ([]events.SystemDeletion, error) {
	systems, err := data.ListDeletedSystems(ctx)
	if err != nil {
		return nil, err
	}
	deletions := make([]events.SystemDeletion, len(systems))
	for i, s := range systems {
		deletions[i] = events.SystemDeletion{ID: s.ID, SystemID: s.SystemID, DeletedAt: s.SourceDeletedAt}
	}
	return deletions, nil
}
