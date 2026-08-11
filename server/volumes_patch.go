package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/proposedchanges"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
)

// patchVolumeRequest's fields are pointers so an absent field (nil) is distinguishable from an
// explicit empty string - only present fields are part of the edit/proposal.
type patchVolumeRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Notes       *string `json:"notes"`
}

type pendingProposalResponse struct {
	ProposalID string `json:"proposalId"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

// Edit a volume, or propose an edit for review.
//
//	@Summary		Edit a volume
//	@Description	admin/editor roles apply the change to the live record; submitter-only roles create a proposed change for review instead.
//	@Tags			volumes
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Volume ID"
//	@Param			request	body		patchVolumeRequest	true	"Fields to change"
//	@Success		200		{object}	interface{}
//	@Success		202		{object}	pendingProposalResponse
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

	diff := patchRequestDiff(req)
	if len(diff) == 0 {
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
	for field, change := range diff {
		change.Old = fieldValue(existing, field)
		diff[field] = change
	}

	roles := authz.Roles(c)
	if authz.HasRole(roles, authz.RoleAdmin) || authz.HasRole(roles, authz.RoleEditor) {
		applyVolumePatch(c, existing, req)
		return
	}

	proposeVolumeChange(c, id, diff)
}

// patchRequestDiff builds the initial diff map from the request's present fields, with New set
// and Old left for the caller to fill in from the live record.
func patchRequestDiff(req patchVolumeRequest) map[string]proposedchanges.FieldChange {
	diff := make(map[string]proposedchanges.FieldChange)
	if req.Title != nil {
		diff["title"] = proposedchanges.FieldChange{New: *req.Title, Status: proposedchanges.StatusPending}
	}
	if req.Description != nil {
		diff["description"] = proposedchanges.FieldChange{New: *req.Description, Status: proposedchanges.StatusPending}
	}
	if req.Notes != nil {
		diff["notes"] = proposedchanges.FieldChange{New: *req.Notes, Status: proposedchanges.StatusPending}
	}
	return diff
}

// fieldValue returns the live volume's current value for one of the patchable fields.
func fieldValue(v *vo.VolumeVO, field string) string {
	switch field {
	case "title":
		return v.Title
	case "description":
		return v.Description
	case "notes":
		return v.Notes
	default:
		return ""
	}
}

// setFieldValue writes value into one of v's patchable fields.
func setFieldValue(v *vo.VolumeVO, field, value string) {
	switch field {
	case "title":
		v.Title = value
	case "description":
		v.Description = value
	case "notes":
		v.Notes = value
	}
}

// applyVolumePatch merges req's present fields into existing and writes the result directly to
// the live record - the admin/editor direct-edit path.
func applyVolumePatch(c *gin.Context, existing *vo.VolumeVO, req patchVolumeRequest) {
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
	updated.UpdatedBy = authz.Subject(c)

	result, err := data.UpdateVolume(c.Request.Context(), existing.ID, &updated)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: err.Error()})
		return
	}
	if result == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	c.Writer.WriteHeader(http.StatusOK)
	if err := jsonapi.MarshalPayload(c.Writer, result); err != nil {
		sentry.CaptureException(err)
	}
}

// proposeVolumeChange stores diff as a pending proposed change rather than touching the live
// record - the submitter-only path.
func proposeVolumeChange(c *gin.Context, volumeID string, diff map[string]proposedchanges.FieldChange) {
	proposal := &proposedchanges.ProposedChange{
		RecordType:  "volume",
		RecordID:    volumeID,
		Diff:        diff,
		SubmittedBy: authz.Subject(c),
	}

	id, err := proposedchanges.Add(c.Request.Context(), proposal)
	if err != nil {
		logging.Logger.Error("Error while storing proposed volume change", "error", err.Error())
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "propose_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, pendingProposalResponse{
		ProposalID: id,
		Status:     proposedchanges.StatusPending,
		Message:    "Change proposed for review",
	})
}
