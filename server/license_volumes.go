package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	apiutil "github.com/sweetrpg/api-core.go/util"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/catalog-objects.go/vo"
)

type patchLicenseVolumesRequest struct {
	VolumeIDs []string `json:"volumeIds"`
}

// fetchLicenseVolumes returns every volume currently referencing a license - mirrors
// getLicenseVolumes' own query (license_ids $in [id]), factored out here since this handler
// needs to call it twice (before and after the diff below).
func fetchLicenseVolumes(c *gin.Context, licenseID string) ([]*vo.VolumeVO, error) {
	inOp := "$in"
	params := apiutil.QueryParams{
		Filter: []apiutil.Filter{{Field: "license_ids", Operation: &inOp, Value: []string{licenseID}}},
	}
	return data.QueryVolumes(c.Request.Context(), params)
}

func fetchLicenseVolumeIDs(c *gin.Context, licenseID string) ([]string, error) {
	volumes, err := fetchLicenseVolumes(c, licenseID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(volumes))
	for i, v := range volumes {
		ids[i] = v.ID
	}
	return ids, nil
}

// patchLicenseVolumes sets the complete list of volumes a license is associated with - full
// replace semantics, same convention volumes_patch.go's own PublisherIDs/StudioIDs already use.
// License<->volume association is volume-owned data (vo.VolumeVO.Licenses), not license-owned -
// getLicenseVolumes/fetchLicenseVolumeIDs above read it via a reverse query for exactly that
// reason. So this diffs the requested set against the current one and, for each added/removed
// volume, updates that volume's own Licenses field directly (editor/admin only - this route
// writes to records other than the one named in the URL, same tier as accept/reject, not open to
// submitters). Not transactional - matches this codebase's existing multi-entity fetch/update
// loops (e.g. applyVolumePatch's own Publisher/Studio ID resolution), which don't use Mongo
// transactions either.
func patchLicenseVolumes(c *gin.Context) {
	id := c.Param("id")

	license, err := data.GetLicense(c.Request.Context(), id)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if license == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	var req patchLicenseVolumesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	currentIDs, err := fetchLicenseVolumeIDs(c, id)
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
		if err := setVolumeHasLicense(c, vid, id, true, updatedBy); err != nil {
			sentry.CaptureException(err)
		}
	}
	for _, vid := range currentIDs {
		if requested[vid] {
			continue
		}
		if err := setVolumeHasLicense(c, vid, id, false, updatedBy); err != nil {
			sentry.CaptureException(err)
		}
	}

	volumes, err := fetchLicenseVolumes(c, id)
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

// setVolumeHasLicense adds or removes one license ID from a volume's Licenses relation and saves
// the volume live - a full-record UpdateVolume, since that's the only write path this data layer
// exposes for volume (matches PublisherIDs/StudioIDs' own "id-only stub is enough, UpdateVolume
// re-resolves the full relation" convention in applyVolumePatch).
func setVolumeHasLicense(c *gin.Context, volumeID, licenseID string, add bool, updatedBy string) error {
	volume, err := data.GetVolume(c.Request.Context(), volumeID)
	if err != nil {
		return err
	}
	if volume == nil {
		return nil
	}

	licenses := make([]*vo.LicenseVO, 0, len(volume.Licenses)+1)
	found := false
	for _, l := range volume.Licenses {
		if l.ID == licenseID {
			found = true
			if !add {
				continue
			}
		}
		licenses = append(licenses, l)
	}
	if add && !found {
		licenses = append(licenses, &vo.LicenseVO{ID: licenseID})
	}
	volume.Licenses = licenses
	volume.UpdatedBy = updatedBy

	_, err = data.UpdateVolume(c.Request.Context(), volumeID, volume, catalogmodels.VersionStateLive)
	return err
}
