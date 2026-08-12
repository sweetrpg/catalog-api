package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-api/proposedchanges"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
)

type acceptProposalRequest struct {
	// Fields lists which changed field names to accept; the rest of the proposal's changed
	// fields are rejected. Nil (the key omitted entirely) means accept every changed field.
	Fields *[]string `json:"fields"`
}

type rejectProposalRequest struct {
	Note string `json:"note"`
}

type reviewProposalResponse struct {
	ProposalID string   `json:"proposalId"`
	Status     string   `json:"status"`
	Applied    []string `json:"applied"`
	Rejected   []string `json:"rejected"`
	Conflicts  []string `json:"conflicts,omitempty"`
}

// List pending proposed changes for a volume.
//
//	@Summary		List a volume's pending proposed changes
//	@Description	editor/admin only.
//	@Tags			volumes
//	@Produce		json
//	@Param			id	path		string	true	"Volume ID"
//	@Success		200	{array}		proposedchanges.ProposedChange
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/proposed-changes [get]
func listVolumeProposedChanges(c *gin.Context) {
	id := c.Param("id")

	proposals, err := proposedchanges.ListPending(c.Request.Context(), "volume", id)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, proposals)
}

// Accept a proposed change, in full or in part.
//
//	@Summary		Accept a proposed volume change
//	@Description	editor/admin only. Omit "fields" to accept every changed field.
//	@Tags			volumes
//	@Accept			json
//	@Produce		json
//	@Param			id			path		string					true	"Volume ID"
//	@Param			proposalId	path		string					true	"Proposed change ID"
//	@Param			request		body		acceptProposalRequest	false	"Fields to accept"
//	@Success		200			{object}	reviewProposalResponse
//	@Failure		404			{object}	apiv.ErrorVO
//	@Failure		409			{object}	apiv.ErrorVO
//	@Failure		500			{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/proposed-changes/{proposalId}/accept [post]
func acceptVolumeProposedChange(c *gin.Context, assetsClient *assets.Client) {
	volumeID := c.Param("id")
	proposalID := c.Param("proposalId")

	var req acceptProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	proposal, ok := loadPendingProposal(c, volumeID, proposalID)
	if !ok {
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

	acceptAll := req.Fields == nil
	acceptSet := map[string]bool{}
	if !acceptAll {
		for _, f := range *req.Fields {
			acceptSet[f] = true
		}
	}

	updated := *existing
	var applied, rejected, conflicts []string
	appliedAny := false

	for field, change := range proposal.Diff {
		if !acceptAll && !acceptSet[field] {
			change.Status = proposedchanges.StatusRejected
			proposal.Diff[field] = change
			rejected = append(rejected, field)
			continue
		}

		if fieldValue(existing, field) != change.Old {
			conflicts = append(conflicts, field)
			continue
		}

		newValue, _ := change.New.(string)
		setFieldValue(&updated, field, newValue)
		change.Status = proposedchanges.StatusAccepted
		proposal.Diff[field] = change
		applied = append(applied, field)
		appliedAny = true
	}

	if proposal.StagedCoverAssetId != "" {
		if err := assetsClient.Promote(c.Request.Context(), "cover-staged", proposal.StagedCoverAssetId, "cover", volumeID); err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "cover_promote_failed", Message: err.Error()})
			return
		}
		updated.CoverAssetId = volumeID
		appliedAny = true
	}
	if len(proposal.StagedSampleAssetIds) > 0 {
		liveSampleIds := make([]string, len(proposal.StagedSampleAssetIds))
		for i, stagedID := range proposal.StagedSampleAssetIds {
			liveID := fmt.Sprintf("%s-%d", volumeID, i)
			if err := assetsClient.Promote(c.Request.Context(), "sample-staged", stagedID, "sample", liveID); err != nil {
				sentry.CaptureException(err)
				c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "sample_promote_failed", Message: err.Error()})
				return
			}
			liveSampleIds[i] = liveID
		}
		updated.SampleAssetIds = liveSampleIds
		appliedAny = true
	}

	if appliedAny {
		updated.UpdatedBy = authz.Subject(c)
		if _, err := data.UpdateVolume(c.Request.Context(), volumeID, &updated); err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: err.Error()})
			return
		}
	}

	finishReview(c, proposal)

	c.JSON(http.StatusOK, reviewProposalResponse{
		ProposalID: proposalID,
		Status:     proposal.Status,
		Applied:    applied,
		Rejected:   rejected,
		Conflicts:  conflicts,
	})
}

// Reject a proposed change in full.
//
//	@Summary		Reject a proposed volume change
//	@Description	editor/admin only.
//	@Tags			volumes
//	@Accept			json
//	@Produce		json
//	@Param			id			path		string					true	"Volume ID"
//	@Param			proposalId	path		string					true	"Proposed change ID"
//	@Param			request		body		rejectProposalRequest	false	"Optional review note"
//	@Success		200			{object}	reviewProposalResponse
//	@Failure		404			{object}	apiv.ErrorVO
//	@Failure		409			{object}	apiv.ErrorVO
//	@Failure		500			{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/proposed-changes/{proposalId}/reject [post]
func rejectVolumeProposedChange(c *gin.Context, assetsClient *assets.Client) {
	volumeID := c.Param("id")
	proposalID := c.Param("proposalId")

	var req rejectProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	proposal, ok := loadPendingProposal(c, volumeID, proposalID)
	if !ok {
		return
	}

	var rejected []string
	for field, change := range proposal.Diff {
		change.Status = proposedchanges.StatusRejected
		proposal.Diff[field] = change
		rejected = append(rejected, field)
	}
	proposal.ReviewNote = req.Note

	if proposal.StagedCoverAssetId != "" {
		if err := assetsClient.Delete(c.Request.Context(), "cover-staged", proposal.StagedCoverAssetId); err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "cover_reclaim_failed", Message: err.Error()})
			return
		}
	}
	for _, stagedID := range proposal.StagedSampleAssetIds {
		if err := assetsClient.Delete(c.Request.Context(), "sample-staged", stagedID); err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "sample_reclaim_failed", Message: err.Error()})
			return
		}
	}

	finishReview(c, proposal)

	c.JSON(http.StatusOK, reviewProposalResponse{
		ProposalID: proposalID,
		Status:     proposal.Status,
		Rejected:   rejected,
	})
}

// Retract a pending proposed change.
//
//	@Summary		Retract a proposed volume change
//	@Description	submitter only, and only their own submissions.
//	@Tags			volumes
//	@Produce		json
//	@Param			id			path		string	true	"Volume ID"
//	@Param			proposalId	path		string	true	"Proposed change ID"
//	@Success		200			{object}	reviewProposalResponse
//	@Failure		403			{object}	apiv.ErrorVO
//	@Failure		404			{object}	apiv.ErrorVO
//	@Failure		409			{object}	apiv.ErrorVO
//	@Failure		500			{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/proposed-changes/{proposalId}/retract [post]
func retractVolumeProposedChange(c *gin.Context) {
	volumeID := c.Param("id")
	proposalID := c.Param("proposalId")

	proposal, ok := loadPendingProposal(c, volumeID, proposalID)
	if !ok {
		return
	}
	if proposal.SubmittedBy != authz.Subject(c) {
		c.JSON(http.StatusForbidden, apiv.ErrorVO{Error: "forbidden", Message: "You can only retract your own submissions"})
		return
	}

	// Not finishReview - retracting isn't a reviewer's decision (no ReviewedBy/DeriveStatus),
	// it's the submitter withdrawing their own pending proposal.
	proposal.Status = proposedchanges.StatusRetracted
	if err := proposedchanges.Update(c.Request.Context(), proposal); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "retract_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, reviewProposalResponse{ProposalID: proposalID, Status: proposal.Status})
}

// pullBackResponse is the response for a successful pull-back.
type pullBackResponse struct {
	RecordID string `json:"recordId"`
	Status   string `json:"status"`
}

// Pull a pending proposed change back into a fresh edit session, retracting it in the process.
//
//	@Summary		Pull a proposed volume change back into an edit session
//	@Description	submitter only, and only their own submissions. Fails with 409 if the caller already has an in-flight session for this record type - same conflict as starting any other session.
//	@Tags			volumes
//	@Produce		json
//	@Param			id			path		string	true	"Volume ID"
//	@Param			proposalId	path		string	true	"Proposed change ID"
//	@Success		200			{object}	pullBackResponse
//	@Failure		403			{object}	apiv.ErrorVO
//	@Failure		404			{object}	apiv.ErrorVO
//	@Failure		409			{object}	apiv.ErrorVO
//	@Failure		500			{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/proposed-changes/{proposalId}/pull-back [post]
func pullBackVolumeProposedChange(c *gin.Context, editSessions *editsession.Store) {
	volumeID := c.Param("id")
	proposalID := c.Param("proposalId")
	userID := authz.Subject(c)

	proposal, ok := loadPendingProposal(c, volumeID, proposalID)
	if !ok {
		return
	}
	if proposal.SubmittedBy != userID {
		c.JSON(http.StatusForbidden, apiv.ErrorVO{Error: "forbidden", Message: "You can only pull back your own submissions"})
		return
	}

	existingSession, err := editSessions.Get(c.Request.Context(), userID, recordTypeVolume)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "session_lookup_failed", Message: err.Error()})
		return
	}
	if existingSession != nil {
		c.JSON(http.StatusConflict, apiv.ErrorVO{
			Error:   "session_exists",
			Message: "You already have an in-flight edit session for this record type - finish or discard it before pulling back a submission",
		})
		return
	}

	fields := make(map[string]any, len(proposal.Diff))
	for field, change := range proposal.Diff {
		fields[field] = change.New
	}
	now := time.Now()
	session := editsession.Session{
		RecordID:           proposal.RecordID,
		Fields:             fields,
		StagedCoverAssetId: proposal.StagedCoverAssetId,
		SampleAssetIds:     proposal.StagedSampleAssetIds,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := editSessions.Set(c.Request.Context(), userID, recordTypeVolume, session); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "session_create_failed", Message: err.Error()})
		return
	}

	proposal.Status = proposedchanges.StatusRetracted
	if err := proposedchanges.Update(c.Request.Context(), proposal); err != nil {
		// Compensate: the session was created but the source proposal couldn't be retracted -
		// "creates the session and retracts the source, or does neither" (task 5.4). Roll the
		// session back out rather than leaving one without the other.
		if delErr := editSessions.Delete(c.Request.Context(), userID, recordTypeVolume); delErr != nil {
			sentry.CaptureException(delErr)
		}
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "pull_back_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, pullBackResponse{RecordID: proposal.RecordID, Status: proposal.Status})
}

// loadPendingProposal fetches a proposal, verifying it belongs to volumeID and is still
// pending. Writes the appropriate error response and returns ok=false otherwise.
func loadPendingProposal(c *gin.Context, volumeID, proposalID string) (*proposedchanges.ProposedChange, bool) {
	proposal, err := proposedchanges.Get(c.Request.Context(), proposalID)
	if err != nil {
		logging.Logger.Warn("Error while loading proposed change", "id", proposalID, "error", err.Error())
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return nil, false
	}
	if proposal == nil || proposal.RecordType != "volume" || proposal.RecordID != volumeID {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return nil, false
	}
	if proposal.Status != proposedchanges.StatusPending {
		c.JSON(http.StatusConflict, apiv.ErrorVO{
			Error:   "already_reviewed",
			Message: "This proposed change has already been reviewed",
		})
		return nil, false
	}
	return proposal, true
}

// finishReview stamps reviewer metadata, re-derives the overall status, and persists the
// proposal. Errors are logged/reported to Sentry but not surfaced separately - the caller has
// already applied any live-record changes and still owes the client a response.
func finishReview(c *gin.Context, proposal *proposedchanges.ProposedChange) {
	now := time.Now()
	proposal.ReviewedBy = authz.Subject(c)
	proposal.ReviewedAt = &now
	proposal.DeriveStatus()

	if err := proposedchanges.Update(c.Request.Context(), proposal); err != nil {
		logging.Logger.Error("Error while persisting proposal review", "id", proposal.ID.Hex(), "error", err.Error())
		sentry.CaptureException(err)
	}
}
