package server

import (
	"context"
	"maps"

	"github.com/gin-contrib/cache/persistence"
	"github.com/sweetrpg/catalog-api/internal/events"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
)

// systemIDsOf extracts the IDs from a volume's system relationship slice.
func systemIDsOf(systems []*vo.SystemVO) []string {
	ids := make([]string, 0, len(systems))
	for _, s := range systems {
		if s != nil {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

// resolveSystemRefs resolves a volume's system references against game-systems-api. It returns
// the references, with each resolved one rewritten to the system's document ID (a caller may
// send the system_id slug) and duplicates dropped, plus the denormalized title map keyed the
// same way. A system that can't be resolved (404 or unreachable) keeps the ID it came with and
// falls back to the caller-supplied hint; with no hint it has no title, which renders the ID and
// is corrected later by the sync consumer. This never returns an error: an unresolvable system
// must not block a volume write. Deleted systems are dropped by the data layer.
func resolveSystemRefs(ctx context.Context, refs []*vo.SystemVO, hint map[string]string) ([]*vo.SystemVO, map[string]string) {
	resolved := make([]*vo.SystemVO, 0, len(refs))
	titles := make(map[string]string, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, requested := range systemIDsOf(refs) {
		id, title := requested, hint[requested]
		sys, err := data.GetSystem(ctx, requested)
		if err == nil && sys != nil {
			if sys.ID != "" {
				id = sys.ID
			}
			if sys.GameSystem != "" {
				title = sys.GameSystem
			}
		}
		if title == "" {
			logging.Logger.Warn("resolveSystemRefs: system title unavailable at write time",
				"system_id", requested, "error", err)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		resolved = append(resolved, &vo.SystemVO{ID: id})
		if title != "" {
			titles[id] = title
		}
	}
	return resolved, titles
}

// mergeTitleHints overlays a request-supplied hint on the volume's existing stored titles, so a
// system whose reference is unchanged keeps its current title if the write-time lookup fails.
func mergeTitleHints(existing, reqHint map[string]string) map[string]string {
	merged := make(map[string]string, len(existing))
	maps.Copy(merged, existing)
	maps.Copy(merged, reqHint)
	return merged
}

// SyncSystemTitle is the handler for gamesystems.events.system.updated: it sets the stored
// title on every volume that references the system and invalidates those volumes' cached reads.
// Idempotent - UpdateVolumeSystemTitleBySystem is a no-op when the title is unchanged, so a
// redelivered event makes no further change. store may be nil (the standalone cmd/consumer runs
// without a cache), in which case invalidation is skipped.
func SyncSystemTitle(store persistence.CacheStore) events.SystemUpdateHandler {
	return func(ctx context.Context, systemID, title string) error {
		recordIDs, err := data.UpdateVolumeSystemTitleBySystem(ctx, systemID, title)
		if err != nil {
			return err
		}
		if store == nil {
			return nil
		}
		paths := []string{"/systems/" + systemID, "/systems/" + systemID + "/volumes"}
		if len(recordIDs) > 0 {
			paths = append(paths, "/volumes")
			for _, id := range recordIDs {
				paths = append(paths, "/volumes/"+id)
			}
		}
		invalidateCachedPaths(store, paths...)
		return nil
	}
}
