package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-api/proposedchanges"
	"github.com/sweetrpg/catalog-data.go/data"
)

// recordTypeVolume is the edit-session recordType this endpoint reads (see docs/
// frontend-conventions.md's edit-session schema in sweetrpg/platform - Volume is the only
// record type the session mechanism supports today).
const recordTypeVolume = "volume"

// Finalize a volume's in-flight edit session.
//
//	@Summary		Finalize a volume edit session
//	@Description	Reads the caller's in-flight edit session for this volume. admin/editor roles apply it directly (promoting any staged cover/samples to live); submitter roles create a proposed change referencing the staged assets without promoting them.
//	@Tags			volumes
//	@Produce		json
//	@Param			id	path		string	true	"Volume ID"
//	@Success		200	{object}	interface{}
//	@Success		202	{object}	pendingProposalResponse
//	@Failure		400	{object}	apiv.ErrorVO
//	@Failure		404	{object}	apiv.ErrorVO
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/finalize-session [post]
func finalizeVolumeSession(c *gin.Context, assetsClient *assets.Client, editSessions *editsession.Store) {
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

		if !applyVolumePatch(c, existing, req) {
			return
		}
		if err := editSessions.Delete(c.Request.Context(), userID, recordTypeVolume); err != nil {
			sentry.CaptureException(err)
		}
		return
	}

	// Submitter: the staged cover/samples ride along on the proposal itself rather than
	// through the generic Diff map (which can't represent them yet, same limitation as
	// req.hasEditorOnlyFields' other cases) - see proposedchanges.ProposedChange's doc comment.
	diff := patchRequestDiff(req)
	for field, change := range diff {
		change.Old = fieldValue(existing, field)
		diff[field] = change
	}

	proposal := &proposedchanges.ProposedChange{
		RecordType:           recordTypeVolume,
		RecordID:             volumeID,
		Diff:                 diff,
		SubmittedBy:          userID,
		StagedCoverAssetId:   session.StagedCoverAssetId,
		StagedSampleAssetIds: session.SampleAssetIds,
	}
	id, err := proposedchanges.Add(c.Request.Context(), proposal)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "propose_failed", Message: err.Error()})
		return
	}

	if err := editSessions.Delete(c.Request.Context(), userID, recordTypeVolume); err != nil {
		sentry.CaptureException(err)
	}

	c.JSON(http.StatusAccepted, pendingProposalResponse{
		ProposalID: id,
		Status:     proposedchanges.StatusPending,
		Message:    "Change proposed for review",
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
