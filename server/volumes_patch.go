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

// patchVolumeRequest's fields are pointers so an absent field (nil) is distinguishable from an
// explicit empty string/omitted array - only present fields are part of the edit/proposal.
//
// Properties/PublisherIDs/StudioIDs/Credits are editor/admin-only for now (see
// applyVolumePatch) - the submitter proposal path's FieldChange.New is applied back via a
// `.(string)` type assertion (volumes_review.go), which can't represent these shapes yet. A
// submitter request touching any of them is rejected with a clear error rather than silently
// dropping the field - extending the proposal review path to handle non-string field types is
// tracked separately, not done here.
//
// Format is editor/admin-only permanently, not as a stopgap - per volume-format-selector's
// spec, a submitter session can't set it at all, directly or via proposal.
type patchVolumeRequest struct {
	Title        *string            `json:"title"`
	Description  *string            `json:"description"`
	Notes        *string            `json:"notes"`
	Properties   *[]propertyRequest `json:"properties"`
	PublisherIDs *[]string          `json:"publisherIds"`
	StudioIDs    *[]string          `json:"studioIds"`
	Credits      *[]creditRequest   `json:"credits"`
	Format       *string            `json:"format"`
}

// hasEditorOnlyFields reports whether req touches a field the submitter-proposal path can't yet
// represent, or that's permanently editor/admin-only (see patchVolumeRequest's doc comment).
func (req patchVolumeRequest) hasEditorOnlyFields() bool {
	return req.Properties != nil || req.PublisherIDs != nil || req.StudioIDs != nil ||
		req.Credits != nil || req.Format != nil
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
	// Properties/PublisherIDs/StudioIDs never populate diff (see hasEditorOnlyFields' doc
	// comment) - without this, an editor's request touching only those fields would 400 here
	// before ever reaching the role check below that's supposed to allow it.
	if len(diff) == 0 && !req.hasEditorOnlyFields() {
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

	if req.hasEditorOnlyFields() {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{
			Error:   "unsupported_field",
			Message: "properties, publisherIds, and studioIds can't be proposed yet - an editor or admin must make this change directly",
		})
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
	updated.UpdatedBy = authz.Subject(c)

	if req.Credits != nil {
		if err := applyCreditsDiff(c, existing.ID, *req.Credits, updated.UpdatedBy); err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: err.Error()})
			return
		}
	}

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
