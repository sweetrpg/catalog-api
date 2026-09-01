package server

import (
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
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
// Properties/PublisherIDs/StudioIDs/SystemIDs/Credits/Format/CoverAssetId/SampleAssetIds are
// editor/admin-only by policy (see hasEditorOnlyFields) - a submitter request touching any of
// them is rejected with a clear error rather than silently dropping the field.
type patchVolumeRequest struct {
	Title          *string            `json:"title"`
	Description    *string            `json:"description"`
	Notes          *string            `json:"notes"`
	Tags           *[]string          `json:"tags"`
	Properties     *[]propertyRequest `json:"properties"`
	PublisherIDs   *[]string          `json:"publisherIds"`
	StudioIDs      *[]string          `json:"studioIds"`
	SystemIDs      *[]string          `json:"systemIds"`
	Credits        *[]creditRequest   `json:"credits"`
	Format         *string            `json:"format"`
	CoverAssetId   *string            `json:"coverAssetId"`
	SampleAssetIds *[]string          `json:"sampleAssetIds"`
}

// hasEditorOnlyFields reports whether req touches a field that's editor/admin-only by policy
// (see patchVolumeRequest's doc comment).
func (req patchVolumeRequest) hasEditorOnlyFields() bool {
	return req.Properties != nil || req.PublisherIDs != nil || req.StudioIDs != nil ||
		req.SystemIDs != nil || req.Credits != nil || req.Format != nil ||
		req.CoverAssetId != nil || req.SampleAssetIds != nil
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
func patchVolume(c *gin.Context, store persistence.CacheStore) {
	id := c.Param("id")
	logging.Logger.Debug("patchVolume: enter", "id", id)

	var req patchVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.Logger.Debug("patchVolume: exit", "id", id, "outcome", "bad_request")
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	// Properties/PublisherIDs/StudioIDs never count as submittable (see hasEditorOnlyFields' doc
	// comment) - without this, an editor's request touching only those fields would 400 here
	// before ever reaching the role check below that's supposed to allow it.
	if !req.hasSubmittableFields() && !req.hasEditorOnlyFields() {
		logging.Logger.Debug("patchVolume: exit", "id", id, "outcome", "bad_request", "reason", "no_fields")
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "no recognized fields to change"})
		return
	}

	existing, err := data.GetVolume(c.Request.Context(), id)
	if err != nil {
		logging.Logger.Error("patchVolume: volume lookup failed", "id", id, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if existing == nil {
		logging.Logger.Debug("patchVolume: exit", "id", id, "outcome", "not_found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	roles := authz.Roles(c)
	isEditor := authz.HasRole(roles, authz.RoleAdmin) || authz.HasRole(roles, authz.RoleEditor)

	if !isEditor && req.hasEditorOnlyFields() {
		logging.Logger.Warn("patchVolume: submitter rejected editor-only fields", "id", id)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{
			Error:   "unsupported_field",
			Message: "properties, publisherIds, studioIds, systemIds, credits, format, coverAssetId, and sampleAssetIds can't be submitted - an editor or admin must make this change directly",
		})
		return
	}

	state := catalogmodels.VersionStateSubmitted
	if isEditor {
		state = catalogmodels.VersionStateLive
	}
	logging.Logger.Info("patchVolume: branch selected", "id", id, "isEditor", isEditor, "state", string(state))
	applyVolumePatch(c, existing, req, state, store)
}

// hasSubmittableFields reports whether req touches a field a submitter is allowed to change
// directly (title/description/notes/tags - tags are free-text labels, same policy as license
// tags) - the fields patchRequestDiff used to diff before the version model made per-field
// diffing unnecessary.
func (req patchVolumeRequest) hasSubmittableFields() bool {
	return req.Title != nil || req.Description != nil || req.Notes != nil || req.Tags != nil
}

// applyVolumePatch merges req's present fields into existing and creates a new version with the
// given state - state VersionStateLive is the admin/editor direct-edit path (the new version
// goes live immediately); VersionStateSubmitted is the submitter path (a new version is created
// for review, the live record is untouched). Returns whether the update succeeded (an HTTP
// response, success or failure, has always already been written either way) - callers that have
// follow-up cleanup contingent on success (finalizeVolumeSession deleting the now-applied
// session) need to know which happened.
func applyVolumePatch(
	c *gin.Context, existing *vo.VolumeVO, req patchVolumeRequest, state catalogmodels.VersionState,
	store persistence.CacheStore,
) bool {
	logging.Logger.Debug("applyVolumePatch: enter", "id", existing.ID, "state", string(state))
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
	if req.Tags != nil {
		tags := make([]modelcore.TagVO, len(*req.Tags))
		for i, name := range *req.Tags {
			tags[i] = modelcore.TagVO{Name: name}
		}
		updated.Tags = tags
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
	if req.SystemIDs != nil {
		systems := make([]*vo.SystemVO, len(*req.SystemIDs))
		for i, id := range *req.SystemIDs {
			systems[i] = &vo.SystemVO{ID: id}
		}
		updated.Systems = systems
	}
	if req.Format != nil {
		updated.Format = *req.Format
	}
	if req.CoverAssetId != nil {
		updated.CoverAssetId = *req.CoverAssetId
	}
	if req.SampleAssetIds != nil {
		if len(*req.SampleAssetIds) > maxSampleAssetIds {
			logging.Logger.Warn("applyVolumePatch: sample limit exceeded", "id", existing.ID, "count", len(*req.SampleAssetIds), "max", maxSampleAssetIds)
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
			logging.Logger.Error("applyVolumePatch: credits diff failed", "id", existing.ID, "error", err)
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: err.Error()})
			return false
		}
	}

	version, err := data.UpdateVolume(c.Request.Context(), existing.ID, &updated, state)
	if err != nil {
		logging.Logger.Error("applyVolumePatch: update failed", "id", existing.ID, "state", string(state), "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: err.Error()})
		return false
	}
	if version == nil {
		logging.Logger.Debug("applyVolumePatch: exit", "id", existing.ID, "outcome", "not_found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return false
	}

	if state != catalogmodels.VersionStateLive {
		logging.Logger.Info("applyVolumePatch: submitted version created", "id", existing.ID, "version", version.Version, "updatedBy", updated.UpdatedBy)
		c.JSON(http.StatusAccepted, submittedVersionResponse{
			Version: version.Version,
			State:   string(version.State),
			Message: "Change submitted for review",
		})
		return true
	}

	logging.Logger.Info("applyVolumePatch: live version applied", "id", existing.ID, "version", version.Version, "updatedBy", updated.UpdatedBy)

	if req.PublisherIDs != nil || req.StudioIDs != nil || req.SystemIDs != nil {
		invalidateVolumeAssociationCache(store, existing, &updated)
	}

	result, err := data.GetVolume(c.Request.Context(), existing.ID)
	if err != nil {
		logging.Logger.Error("applyVolumePatch: post-update query failed", "id", existing.ID, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return false
	}

	if volumeEventPublisher != nil && result != nil {
		volumeEventPublisher.PublishEntityUpdated(c.Request.Context(), "volume", result.ID, version.Version, map[string]interface{}{
			"title": result.Title,
		})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	c.Writer.WriteHeader(http.StatusOK)
	if err := jsonapi.MarshalPayload(c.Writer, result); err != nil {
		sentry.CaptureException(err)
	}
	logging.Logger.Debug("applyVolumePatch: exit", "id", existing.ID, "outcome", "ok")
	return true
}

// applyCreditsDiff reconciles a volume's contribution credits against desired - the full,
// intended set of (person, role) pairs - deleting existing contributions no longer present and
// adding new ones. Contributions are single-role (one document per person/volume/role).
func applyCreditsDiff(c *gin.Context, volumeID string, desired []creditRequest, updatedBy string) error {
	logging.Logger.Debug("applyCreditsDiff: enter", "volumeId", volumeID, "desired", len(desired), "updatedBy", updatedBy)
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

	have := make(map[creditKey]string, len(existing))
	for _, contribution := range existing {
		have[creditKey{contribution.Person.ID, contribution.Role}] = contribution.ID
		if !wanted[creditKey{contribution.Person.ID, contribution.Role}] {
			if _, err := data.DeleteContribution(c.Request.Context(), contribution.ID); err != nil {
				return err
			}
		}
	}

	for key := range wanted {
		if _, ok := have[key]; ok {
			continue
		}
		if _, err := data.AddContribution(c.Request.Context(), key.personID, volumeID, key.roleType, updatedBy); err != nil {
			return err
		}
	}
	logging.Logger.Debug("applyCreditsDiff: exit", "volumeId", volumeID, "outcome", "ok")
	return nil
}
