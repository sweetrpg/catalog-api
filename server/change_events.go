package server

import (
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/common.go/logging"
)

// publishVolumeUpdated announces a committed change to a live volume. The publisher is nil-safe
// and fail-open.
func publishVolumeUpdated(c *gin.Context, id, title string, revision int) {
	volumeEventPublisher.PublishEntityUpdated(c.Request.Context(), "volume", id, revision, map[string]interface{}{
		"title": title,
	})
}

// publishCurrentVolume announces a volume's live state after a change that doesn't return it
// (restore, rollback). A failed lookup drops the event; the write has already committed.
func publishCurrentVolume(c *gin.Context, id string) {
	volume, err := data.GetVolume(c.Request.Context(), id)
	if err != nil || volume == nil {
		logging.Logger.Error("volume.updated event dropped: volume lookup failed", "entity_id", id, "error", err)
		return
	}
	versions, err := data.ListVolumeVersions(c.Request.Context(), id)
	if err != nil {
		logging.Logger.Error("volume.updated event dropped: version lookup failed", "entity_id", id, "error", err)
		return
	}
	for _, v := range versions {
		if string(v.State) == string(catalogmodels.VersionStateLive) {
			publishVolumeUpdated(c, id, volume.Title, v.Version)
			return
		}
	}
	logging.Logger.Error("volume.updated event dropped: no live version", "entity_id", id)
}

// publishCurrentEntity is publishCurrentVolume for the generic version-model entities. action is
// "created" or "updated"; a revision of 0 means "look up the live version number".
func publishCurrentEntity[T any, V any](c *gin.Context, cfg entityVersionAPIConfig[T, V], id, action string, revision int) {
	if cfg.onPublishEvent == nil {
		return
	}
	entity, err := cfg.get(c, id)
	if err != nil || entity == nil {
		logging.Logger.Error("entity event dropped: lookup failed", "entity_type", cfg.recordType, "entity_id", id, "action", action, "error", err)
		return
	}
	if revision == 0 {
		versions, err := cfg.listVersions(c, id)
		if err != nil {
			logging.Logger.Error("entity event dropped: version lookup failed", "entity_type", cfg.recordType, "entity_id", id, "action", action, "error", err)
			return
		}
		for _, v := range versions {
			if cfg.versionState(v) == string(catalogmodels.VersionStateLive) {
				revision = cfg.versionNumber(v)
				break
			}
		}
	}
	cfg.onPublishEvent(c, id, revision, action, *entity)
}
