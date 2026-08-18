package server

import (
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/catalog-objects.go/vo"
	modelcore "github.com/sweetrpg/model-core.go/vo"
)

// propertyRequest mirrors modelcore.PropertyVO for JSON binding.
type propertyRequest struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// creditRequest is one desired (person, contribution type) credit. Credits is a full-replace
// list, diffed against the volume's existing contributions in applyVolumePatch - same semantics
// as PublisherIDs/StudioIDs.
type creditRequest struct {
	PersonID         string `json:"personId"`
	ContributionType string `json:"contributionType"`
}

// maxSampleAssetIds is the cap on a volume's sample images (volume-sample-pages spec).
const maxSampleAssetIds = 5

// patchVolumeRequest's fields are pointers so an absent field (nil) is distinguishable from an
// explicit empty string/omitted array - only present fields are part of the edit/submission.
//
// Properties/PublisherIDs/StudioIDs/Credits/Format/CoverAssetId/SampleAssetIds are editor/admin
// -only by policy (see hasEditorOnlyFields) - a submitter request touching any of them is
// rejected with a clear error rather than silently dropping the field.
type patchVolumeRequest struct {
	Title          *string            `json:"title"`
	Description    *string            `json:"description"`
	Notes          *string            `json:"notes"`
	Properties     *[]propertyRequest `json:"properties"`
	PublisherIDs   *[]string          `json:"publisherIds"`
	StudioIDs      *[]string          `json:"studioIds"`
	Credits        *[]creditRequest   `json:"credits"`
	Format         *string            `json:"format"`
	CoverAssetId   *string            `json:"coverAssetId"`
	SampleAssetIds *[]string          `json:"sampleAssetIds"`
}

// hasEditorOnlyFields reports whether req touches a field that's editor/admin-only by policy
// (see patchVolumeRequest's doc comment).
func (req patchVolumeRequest) hasEditorOnlyFields() bool {
	return req.Properties != nil || req.PublisherIDs != nil || req.StudioIDs != nil ||
		req.Credits != nil || req.Format != nil || req.CoverAssetId != nil || req.SampleAssetIds != nil
}

// submittedVersionResponse is returned when a submitter's PATCH creates a submitted version
// rather than editing the live record.
type submittedVersionResponse struct {
	Version int    `json:"version"`
	State   string `json:"state"`
	Message string `json:"message"`
}

// Edit a volume, or submit an edit for review.
//
//	@Summary		Edit a volume
//	@Description	admin/editor roles create a new version that goes live immediately; submitter-only roles create a submitted version for review instead.
//	@Tags			volumes
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Volume ID"
//	@Param			request	body		patchVolumeRequest	true	"Fields to change"
//	@Success		200		{object}	interface{}
//	@Success		202		{object}	submittedVersionResponse
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/volumes/{id} [patch]
func patchVolume(c *gin.Context) {
	id := c.Param("id")

	var req patchVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	// Properties/PublisherIDs/StudioIDs never count as submittable (see hasEditorOnlyFields' doc
	// comment) - without this, an editor's request touching only those fields would 400 here
	// before ever reaching the role check below that's supposed to allow it.
	if !req.hasSubmittableFields() && !req.hasEditorOnlyFields() {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "no recognized fields to change"})
		return
	}

	existing, err := data.GetVolume(c.Request.Context(), id)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	roles := authz.Roles(c)
	isEditor := authz.HasRole(roles, authz.RoleAdmin) || authz.HasRole(roles, authz.RoleEditor)

	if !isEditor && req.hasEditorOnlyFields() {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{
			Error:   "unsupported_field",
			Message: "properties, publisherIds, studioIds, credits, format, coverAssetId, and sampleAssetIds can't be submitted - an editor or admin must make this change directly",
		})
		return
	}

	state := catalogmodels.VersionStateSubmitted
	if isEditor {
		state = catalogmodels.VersionStateLive
	}
	applyVolumePatch(c, existing, req, state)
}

// hasSubmittableFields reports whether req touches a field a submitter is allowed to change
// directly (title/description/notes) - the fields patchRequestDiff used to diff before the
// version model made per-field diffing unnecessary.
func (req patchVolumeRequest) hasSubmittableFields() bool {
	return req.Title != nil || req.Description != nil || req.Notes != nil
}

// applyVolumePatch merges req's present fields into existing and creates a new version with the
// given state - state VersionStateLive is the admin/editor direct-edit path (the new version
// goes live immediately); VersionStateSubmitted is the submitter path (a new version is created
// for review, the live record is untouched). Returns whether the update succeeded (an HTTP
// response, success or failure, has always already been written either way) - callers that have
// follow-up cleanup contingent on success (finalizeVolumeSession deleting the now-applied
// session) need to know which happened.
func applyVolumePatch(c *gin.Context, existing *vo.VolumeVO, req patchVolumeRequest, state catalogmodels.VersionState) bool {
	updated := *existing
	if req.Title != nil {
		updated.Title = *req.Title
	}
	if req.Description != nil {
		updated.Description = *req.Description
	}
	if req.Notes != nil {
		updated.Notes = *req.Notes
	}
	if req.Properties != nil {
		props := make([]modelcore.PropertyVO, len(*req.Properties))
		for i, p := range *req.Properties {
			props[i] = modelcore.PropertyVO{Name: p.Name, Kind: p.Kind, Value: p.Value}
		}
		updated.Properties = props
	}
	if req.PublisherIDs != nil {
		publishers := make([]*vo.PublisherVO, len(*req.PublisherIDs))
		for i, id := range *req.PublisherIDs {
			publishers[i] = &vo.PublisherVO{ID: id}
		}
		updated.Publishers = publishers
	}
	if req.StudioIDs != nil {
		studios := make([]*vo.StudioVO, len(*req.StudioIDs))
		for i, id := range *req.StudioIDs {
			studios[i] = &vo.StudioVO{ID: id}
		}
		updated.Studios = studios
	}
	if req.Format != nil {
		updated.Format = *req.Format
	}
	if req.CoverAssetId != nil {
		updated.CoverAssetId = *req.CoverAssetId
	}
	if req.SampleAssetIds != nil {
		if len(*req.SampleAssetIds) > maxSampleAssetIds {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{
				Error:   "sample_limit_exceeded",
				Message: fmt.Sprintf("a volume may have at most %d samples", maxSampleAssetIds),
			})
			return false
		}
		updated.SampleAssetIds = *req.SampleAssetIds
	}
	updated.UpdatedBy = authz.Subject(c)

	if req.Credits != nil {
		if err := applyCreditsDiff(c, existing.ID, *req.Credits, updated.UpdatedBy); err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: err.Error()})
			return false
		}
	}

	version, err := data.UpdateVolume(c.Request.Context(), existing.ID, &updated, state)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: err.Error()})
		return false
	}
	if version == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return false
	}

	if state != catalogmodels.VersionStateLive {
		c.JSON(http.StatusAccepted, submittedVersionResponse{
			Version: version.Version,
			State:   string(version.State),
			Message: "Change submitted for review",
		})
		return true
	}

	result, err := data.GetVolume(c.Request.Context(), existing.ID)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return false
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	c.Writer.WriteHeader(http.StatusOK)
	if err := jsonapi.MarshalPayload(c.Writer, result); err != nil {
		sentry.CaptureException(err)
	}
	return true
}

// applyCreditsDiff reconciles a volume's contribution credits against desired - the full,
// intended set of (person, contribution type) pairs - deleting existing single-role
// contributions no longer present and adding new ones. Only single-role contributions
// (Roles == [type]) are considered part of this mechanism; a contribution with multiple roles
// (predating credits, or created some other way) is left untouched even if none of its roles
// match, since it isn't representable as one desired pair.
func applyCreditsDiff(c *gin.Context, volumeID string, desired []creditRequest, updatedBy string) error {
	existing, err := data.QueryContributionsByVolume(c.Request.Context(), volumeID)
	if err != nil {
		return err
	}

	type creditKey struct {
		personID string
		roleType string
	}
	wanted := make(map[creditKey]bool, len(desired))
	for _, cr := range desired {
		wanted[creditKey{cr.PersonID, cr.ContributionType}] = true
	}

	have := make(map[creditKey]bool, len(existing))
	for _, contribution := range existing {
		if len(contribution.Roles) != 1 {
			continue
		}
		key := creditKey{contribution.Person.ID, contribution.Roles[0]}
		have[key] = true
		if !wanted[key] {
			if _, err := data.DeleteContribution(c.Request.Context(), contribution.ID); err != nil {
				return err
			}
		}
	}

	for key := range wanted {
		if have[key] {
			continue
		}
		if _, err := data.AddContribution(c.Request.Context(), key.personID, volumeID, []string{key.roleType}, updatedBy); err != nil {
			return err
		}
	}
	return nil
}
