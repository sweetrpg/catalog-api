package server

import (
	"context"

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

// resolveSystemTitles builds the denormalized title map for a volume's system references,
// resolving each against game-systems-api. A system that can't be resolved (404 or unreachable)
// falls back to the caller-supplied hint, and if there is no hint it is left unset - stored as
// an empty title, which renders the ID and is corrected later by the sync consumer. This never
// returns an error: an unresolvable system must not block a volume write.
func resolveSystemTitles(ctx context.Context, systemIDs []string, hint map[string]string) map[string]string {
	titles := make(map[string]string, len(systemIDs))
	for _, id := range systemIDs {
		sys, err := data.GetSystem(ctx, id)
		switch {
		case err == nil && sys != nil && sys.GameSystem != "":
			titles[id] = sys.GameSystem
		case hint[id] != "":
			titles[id] = hint[id]
		default:
			logging.Logger.Warn("resolveSystemTitles: system title unavailable at write time",
				"system_id", id, "error", err)
		}
	}
	return titles
}

// mergeTitleHints overlays a request-supplied hint on the volume's existing stored titles, so a
// system whose reference is unchanged keeps its current title if the write-time lookup fails.
func mergeTitleHints(existing, reqHint map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(reqHint))
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range reqHint {
		merged[k] = v
	}
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
