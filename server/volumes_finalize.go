package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-api/submissioncap"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-objects.go/models"
)

// recordTypeVolume is the edit-session recordType this endpoint reads (see docs/
// frontend-conventions.md's edit-session schema in sweetrpg/platform - Volume is the only
// record type the session mechanism supports today).
const recordTypeVolume = "volume"

// Finalize a volume's in-flight edit session.
//
//	@Summary		Finalize a volume edit session
//	@Description	Reads the caller's in-flight edit session for this volume. admin/editor roles apply it directly (promoting any staged cover/samples to live); submitter roles create a submitted version referencing the staged assets without promoting them.
//	@Tags			volumes
//	@Produce		json
//	@Param			id	path		string	true	"Volume ID"
//	@Success		200	{object}	interface{}
//	@Success		202	{object}	submittedVersionResponse
//	@Failure		400	{object}	apiv.ErrorVO
//	@Failure		404	{object}	apiv.ErrorVO
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/finalize-session [post]
func finalizeVolumeSession(
	c *gin.Context, assetsClient *assets.Client, editSessions *editsession.Store,
	store persistence.CacheStore,
) {
	volumeID := c.Param("id")
	userID := authz.Subject(c)

	session, err := editSessions.Get(c.Request.Context(), userID, recordTypeVolume)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "session_lookup_failed", Message: err.Error()})
		return
	}
	if session == nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "no_session", Message: "No in-flight edit session for this volume"})
		return
	}
	if session.RecordID != volumeID {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "session_mismatch", Message: "The in-flight session belongs to a different volume"})
		return
	}

	existing, err := data.GetVolume(c.Request.Context(), volumeID)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	// session.Fields round-trips through patchVolumeRequest's own JSON tags (title,
	// description, notes, properties, publisherIds, studioIds, credits, format) rather than
	// hand-unmarshaling each key - the two shapes are deliberately the same, since finalize
	// reuses the PATCH role-branch below instead of introducing a second code path.
	req, err := fieldsToPatchRequest(session.Fields)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_session", Message: err.Error()})
		return
	}

	roles := authz.Roles(c)
	if authz.HasRole(roles, authz.RoleAdmin) || authz.HasRole(roles, authz.RoleEditor) {
		if session.StagedCoverAssetId != "" {
			if err := assetsClient.Promote(c.Request.Context(), "cover-staged", session.StagedCoverAssetId, "cover", volumeID); err != nil {
				sentry.CaptureException(err)
				c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "cover_promote_failed", Message: err.Error()})
				return
			}
			coverID := volumeID
			req.CoverAssetId = &coverID
		}
		if len(session.SampleAssetIds) > 0 {
			liveSampleIds := make([]string, len(session.SampleAssetIds))
			for i, stagedID := range session.SampleAssetIds {
				liveID := fmt.Sprintf("%s-%d", volumeID, i)
				if err := assetsClient.Promote(c.Request.Context(), "sample-staged", stagedID, "sample", liveID); err != nil {
					sentry.CaptureException(err)
					c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "sample_promote_failed", Message: err.Error()})
					return
				}
				liveSampleIds[i] = liveID
			}
			req.SampleAssetIds = &liveSampleIds
		}

		if !applyVolumePatch(c, existing, req, models.VersionStateLive, store) {
			return
		}
		if err := editSessions.Delete(c.Request.Context(), userID, recordTypeVolume); err != nil {
			sentry.CaptureException(err)
		}
		return
	}

	// Submitter: check the unapproved-submission cap before creating anything - a rejected
	// finalize must not touch the session (existing.RecordID stays theirs to retry) or count
	// against the cap itself.
	capValue, err := submissioncap.CapFor(c.Request.Context(), userID)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "cap_lookup_failed", Message: err.Error()})
		return
	}
	pendingCount, err := data.CountSubmittedVolumeVersionsBySubmitter(c.Request.Context(), userID)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "cap_lookup_failed", Message: err.Error()})
		return
	}
	if int(pendingCount) >= capValue {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{
			Error:   "submission_cap_reached",
			Message: fmt.Sprintf("You have %d pending submissions, at your cap of %d - retract one before finalizing another", pendingCount, capValue),
		})
		return
	}

	// The staged cover/samples ride along on the submitted version as StagedCoverAssetId/
	// StagedSampleAssetIds rather than being promoted now - see design.md's "Staged edit-session
	// assets ride on the submitted version as separate staged fields". existing's own
	// CoverAssetId/SampleAssetIds are left untouched (still the current live values) since
	// nothing has been promoted yet.
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
	updated.UpdatedBy = userID

	var stagedCoverAssetId *string
	if session.StagedCoverAssetId != "" {
		stagedCoverAssetId = &session.StagedCoverAssetId
	}

	version, err := data.CreateSubmittedVolumeVersionWithStagedAssets(c.Request.Context(), volumeID, &updated, userID, stagedCoverAssetId, session.SampleAssetIds)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "propose_failed", Message: err.Error()})
		return
	}
	if version == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	if err := editSessions.Delete(c.Request.Context(), userID, recordTypeVolume); err != nil {
		sentry.CaptureException(err)
	}

	c.JSON(http.StatusAccepted, submittedVersionResponse{
		Version: version.Version,
		State:   string(version.State),
		Message: "Change submitted for review",
	})
}

// fieldsToPatchRequest decodes a session's Fields map into the same shape patchVolumeRequest
// binds from a PATCH body, via a JSON round-trip rather than per-key type assertions.
func fieldsToPatchRequest(fields map[string]any) (patchVolumeRequest, error) {
	raw, err := json.Marshal(fields)
	if err != nil {
		return patchVolumeRequest{}, fmt.Errorf("marshal session fields: %w", err)
	}
	var req patchVolumeRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return patchVolumeRequest{}, fmt.Errorf("unmarshal session fields: %w", err)
	}
	return req, nil
}
