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

type patchPublisherVolumesRequest struct {
	VolumeIDs []string `json:"volumeIds"`
}

// fetchPublisherVolumesForPatch returns every volume currently referencing a publisher - mirrors
// getPublisherVolumes' own query (publisher_ids $in [id]), factored out here since this handler
// needs to call it twice (before and after the diff below).
func fetchPublisherVolumesForPatch(c *gin.Context, publisherID string) ([]*vo.VolumeVO, error) {
	inOp := "$in"
	params := apiutil.QueryParams{
		Filter: []apiutil.Filter{
			{Field: "publisher_ids", Operation: &inOp, Value: []string{publisherID}},
		},
	}
	return data.QueryVolumes(c.Request.Context(), params)
}

func fetchPublisherVolumeIDs(c *gin.Context, publisherID string) ([]string, error) {
	volumes, err := fetchPublisherVolumesForPatch(c, publisherID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(volumes))
	for i, v := range volumes {
		ids[i] = v.ID
	}
	return ids, nil
}

// patchPublisherVolumes sets the complete list of volumes a publisher is associated with - full
// replace semantics, same convention patchStudioVolumes/patchLicenseVolumes use: publisher<->
// volume association is volume-owned data, not publisher-owned, so this diffs the requested set
// against the current one and updates each added/removed volume's own Publishers field directly.
// Editor/admin only, same tier as patchStudioVolumes - this route writes to records other than
// the one named in the URL.
func patchPublisherVolumes(c *gin.Context, store persistence.CacheStore) {
	id := c.Param("id")

	publisher, err := data.GetPublisher(c.Request.Context(), id)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if publisher == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	var req patchPublisherVolumesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	currentIDs, err := fetchPublisherVolumeIDs(c, id)
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
		if err := setVolumeHasPublisher(c, vid, id, true, updatedBy); err != nil {
			sentry.CaptureException(err)
		}
	}
	for _, vid := range currentIDs {
		if requested[vid] {
			continue
		}
		if err := setVolumeHasPublisher(c, vid, id, false, updatedBy); err != nil {
			sentry.CaptureException(err)
		}
	}

	invalidatePublisherVolumesCache(store, id, currentIDs, req.VolumeIDs)

	volumes, err := fetchPublisherVolumesForPatch(c, id)
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

// setVolumeHasPublisher adds or removes one publisher ID from a volume's Publishers relation and
// saves the volume live - a full-record UpdateVolume, since that's the only write path this data
// layer exposes for volume (mirrors setVolumeHasStudio/setVolumeHasLicense).
func setVolumeHasPublisher(c *gin.Context, volumeID, publisherID string, add bool, updatedBy string) error {
	volume, err := data.GetVolume(c.Request.Context(), volumeID)
	if err != nil {
		return err
	}
	if volume == nil {
		return nil
	}

	publishers := make([]*vo.PublisherVO, 0, len(volume.Publishers)+1)
	found := false
	for _, p := range volume.Publishers {
		if p.ID == publisherID {
			found = true
			if !add {
				continue
			}
		}
		publishers = append(publishers, p)
	}
	if add && !found {
		publishers = append(publishers, &vo.PublisherVO{ID: publisherID})
	}
	volume.Publishers = publishers
	volume.UpdatedBy = updatedBy

	_, err = data.UpdateVolume(c.Request.Context(), volumeID, volume, catalogmodels.VersionStateLive)
	return err
}
