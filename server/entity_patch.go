package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/proposedchanges"
	"github.com/sweetrpg/common.go/logging"
)

// entityFieldAccessor reads/writes one patchable string field on an entity VO.
type entityFieldAccessor[T any] struct {
	get func(*T) string
	set func(*T, string)
}

// entityPatchConfig wires the generic patch/proposal/review flow below (shared across
// publishers, studios, persons, and licenses - the same submitter/editor/admin
// direct-edit-vs-proposal pattern volumes_patch.go/volumes_review.go implement for volumes) to
// one entity type's real Get/Update calls and patchable field accessors. See design.md's "share
// a generic proposed-change helper" decision - this avoids reimplementing the diff/conflict/
// partial-accept logic four times.
type entityPatchConfig[T any] struct {
	recordType   string
	get          func(c *gin.Context, id string) (*T, error)
	update       func(c *gin.Context, id string, entity *T) (*T, error)
	setUpdatedBy func(*T, string)
	fields       map[string]entityFieldAccessor[T]
}

// patchEntity edits an entity, or proposes an edit for review, depending on the caller's role -
// the generic counterpart of patchVolume.
func patchEntity[T any](cfg entityPatchConfig[T]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req map[string]*string
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
			return
		}

		diff := make(map[string]proposedchanges.FieldChange)
		for field, value := range req {
			if _, known := cfg.fields[field]; !known || value == nil {
				continue
			}
			diff[field] = proposedchanges.FieldChange{New: *value, Status: proposedchanges.StatusPending}
		}
		if len(diff) == 0 {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "no recognized fields to change"})
			return
		}

		existing, err := cfg.get(c, id)
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
			change.Old = cfg.fields[field].get(existing)
			diff[field] = change
		}

		roles := authz.Roles(c)
		if authz.HasRole(roles, authz.RoleAdmin) || authz.HasRole(roles, authz.RoleEditor) {
			applyEntityPatch(c, cfg, id, existing, req)
			return
		}

		proposeEntityChange(c, cfg.recordType, id, diff)
	}
}

// applyEntityPatch merges req's present, known fields into existing and writes the result
// directly to the live record - the admin/editor direct-edit path.
func applyEntityPatch[T any](c *gin.Context, cfg entityPatchConfig[T], id string, existing *T, req map[string]*string) {
	updated := *existing
	for field, value := range req {
		accessor, known := cfg.fields[field]
		if !known || value == nil {
			continue
		}
		accessor.set(&updated, *value)
	}
	cfg.setUpdatedBy(&updated, authz.Subject(c))

	result, err := cfg.update(c, id, &updated)
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

// proposeEntityChange stores diff as a pending proposed change rather than touching the live
// record - the submitter-only path.
func proposeEntityChange(c *gin.Context, recordType, id string, diff map[string]proposedchanges.FieldChange) {
	proposal := &proposedchanges.ProposedChange{
		RecordType:  recordType,
		RecordID:    id,
		Diff:        diff,
		SubmittedBy: authz.Subject(c),
	}

	pid, err := proposedchanges.Add(c.Request.Context(), proposal)
	if err != nil {
		logging.Logger.Error("Error while storing proposed change", "recordType", recordType, "error", err.Error())
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "propose_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, pendingProposalResponse{
		ProposalID: pid,
		Status:     proposedchanges.StatusPending,
		Message:    "Change proposed for review",
	})
}

// listEntityProposedChanges lists a record's pending proposed changes - editor/admin only.
func listEntityProposedChanges(recordType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		proposals, err := proposedchanges.ListPending(c.Request.Context(), recordType, id)
		if err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
			return
		}

		c.JSON(http.StatusOK, proposals)
	}
}

// acceptEntityProposedChange accepts a proposed change, in full or in part - the generic
// counterpart of acceptVolumeProposedChange.
func acceptEntityProposedChange[T any](cfg entityPatchConfig[T]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		proposalID := c.Param("proposalId")

		var req acceptProposalRequest
		if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
			return
		}

		proposal, ok := loadPendingProposalFor(c, cfg.recordType, id, proposalID)
		if !ok {
			return
		}

		existing, err := cfg.get(c, id)
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
			accessor, known := cfg.fields[field]
			if !known {
				continue
			}
			if !acceptAll && !acceptSet[field] {
				change.Status = proposedchanges.StatusRejected
				proposal.Diff[field] = change
				rejected = append(rejected, field)
				continue
			}

			if accessor.get(existing) != change.Old {
				conflicts = append(conflicts, field)
				continue
			}

			newValue, _ := change.New.(string)
			accessor.set(&updated, newValue)
			change.Status = proposedchanges.StatusAccepted
			proposal.Diff[field] = change
			applied = append(applied, field)
			appliedAny = true
		}

		if appliedAny {
			cfg.setUpdatedBy(&updated, authz.Subject(c))
			if _, err := cfg.update(c, id, &updated); err != nil {
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
}

// rejectEntityProposedChange rejects a proposed change in full - the generic counterpart of
// rejectVolumeProposedChange.
func rejectEntityProposedChange(recordType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		proposalID := c.Param("proposalId")

		var req rejectProposalRequest
		if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
			return
		}

		proposal, ok := loadPendingProposalFor(c, recordType, id, proposalID)
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

		finishReview(c, proposal)

		c.JSON(http.StatusOK, reviewProposalResponse{
			ProposalID: proposalID,
			Status:     proposal.Status,
			Rejected:   rejected,
		})
	}
}

// loadPendingProposalFor fetches a proposal, verifying it belongs to (recordType, recordID) and
// is still pending. Writes the appropriate error response and returns ok=false otherwise - the
// generic counterpart of volumes_review.go's loadPendingProposal (that one stays volume-only
// rather than being rewritten in terms of this, to avoid touching working volume behavior).
func loadPendingProposalFor(c *gin.Context, recordType, recordID, proposalID string) (*proposedchanges.ProposedChange, bool) {
	proposal, err := proposedchanges.Get(c.Request.Context(), proposalID)
	if err != nil {
		logging.Logger.Warn("Error while loading proposed change", "id", proposalID, "error", err.Error())
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return nil, false
	}
	if proposal == nil || proposal.RecordType != recordType || proposal.RecordID != recordID {
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
