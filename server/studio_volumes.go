package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	apiutil "github.com/sweetrpg/api-core.go/util"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/catalog-objects.go/vo"
)

type patchStudioVolumesRequest struct {
	VolumeIDs []string `json:"volumeIds"`
}

// fetchStudioVolumesForPatch returns every volume currently referencing a studio - mirrors
// getStudioVolumes' own query (studio_ids $in [id]), factored out here since this handler needs
// to call it twice (before and after the diff below).
func fetchStudioVolumesForPatch(c *gin.Context, studioID string) ([]*vo.VolumeVO, error) {
	inOp := "$in"
	params := apiutil.QueryParams{
		Filter: []apiutil.Filter{{Field: "studio_ids", Operation: &inOp, Value: []string{studioID}}},
	}
	return data.QueryVolumes(c.Request.Context(), params)
}

func fetchStudioVolumeIDs(c *gin.Context, studioID string) ([]string, error) {
	volumes, err := fetchStudioVolumesForPatch(c, studioID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(volumes))
	for i, v := range volumes {
		ids[i] = v.ID
	}
	return ids, nil
}

// patchStudioVolumes sets the complete list of volumes a studio is associated with - full
// replace semantics, same convention patchLicenseVolumes uses (see license_volumes.go's own
// comment for the rationale: studio<->volume association is volume-owned data, not
// studio-owned, so this diffs the requested set against the current one and updates each
// added/removed volume's own Studios field directly). Editor/admin only, same tier as
// patchLicenseVolumes - this route writes to records other than the one named in the URL.
func patchStudioVolumes(c *gin.Context, store persistence.CacheStore) {
	id := c.Param("id")

	studio, err := data.GetStudio(c.Request.Context(), id)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if studio == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	var req patchStudioVolumesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	currentIDs, err := fetchStudioVolumeIDs(c, id)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}

	current := make(map[string]bool, len(currentIDs))
	for _, vid := range currentIDs {
		current[vid] = true
	}
	requested := make(map[string]bool, len(req.VolumeIDs))
	for _, vid := range req.VolumeIDs {
		requested[vid] = true
	}

	updatedBy := authz.Subject(c)

	for _, vid := range req.VolumeIDs {
		if current[vid] {
			continue
		}
		if err := setVolumeHasStudio(c, vid, id, true, updatedBy); err != nil {
			sentry.CaptureException(err)
		}
	}
	for _, vid := range currentIDs {
		if requested[vid] {
			continue
		}
		if err := setVolumeHasStudio(c, vid, id, false, updatedBy); err != nil {
			sentry.CaptureException(err)
		}
	}

	invalidateStudioVolumesCache(store, id, currentIDs, req.VolumeIDs)

	volumes, err := fetchStudioVolumesForPatch(c, id)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	c.Writer.WriteHeader(http.StatusOK)
	if err := jsonapi.MarshalPayload(c.Writer, volumes); err != nil {
		sentry.CaptureException(err)
	}
}

// setVolumeHasStudio adds or removes one studio ID from a volume's Studios relation and saves
// the volume live - a full-record UpdateVolume, since that's the only write path this data layer
// exposes for volume (mirrors setVolumeHasLicense).
func setVolumeHasStudio(c *gin.Context, volumeID, studioID string, add bool, updatedBy string) error {
	volume, err := data.GetVolume(c.Request.Context(), volumeID)
	if err != nil {
		return err
	}
	if volume == nil {
		return nil
	}

	studios := make([]*vo.StudioVO, 0, len(volume.Studios)+1)
	found := false
	for _, s := range volume.Studios {
		if s.ID == studioID {
			found = true
			if !add {
				continue
			}
		}
		studios = append(studios, s)
	}
	if add && !found {
		studios = append(studios, &vo.StudioVO{ID: studioID})
	}
	volume.Studios = studios
	volume.UpdatedBy = updatedBy

	_, err = data.UpdateVolume(c.Request.Context(), volumeID, volume, catalogmodels.VersionStateLive)
	return err
}
