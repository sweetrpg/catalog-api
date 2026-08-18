package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-data.go/data"
)

// acceptVersionRequest mirrors acceptProposalRequest for the version-based accept action.
type acceptVersionRequest struct {
	// Fields lists which changed field names to accept; the rest are left unaccepted. Nil (the
	// key omitted entirely) means accept every changed field.
	Fields *[]string `json:"fields"`
}

type rejectVersionRequest struct {
	Note string `json:"note"`
}

type reviewVersionResponse struct {
	Version   int      `json:"version"`
	State     string   `json:"state"`
	Conflicts []string `json:"conflicts,omitempty"`
}

// versionParam parses the :version path parameter, writing a 400 response and returning ok=false
// if it isn't a positive integer.
func versionParam(c *gin.Context) (int, bool) {
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version < 1 {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "version must be a positive integer"})
		return 0, false
	}
	return version, true
}

// List a volume's version history.
//
//	@Summary		List a volume's versions
//	@Description	Every version for this volume, newest first.
//	@Tags			volumes
//	@Produce		json
//	@Param			id	path		string	true	"Volume ID"
//	@Success		200	{array}		vo.VolumeVersionVO
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/versions [get]
func listVolumeVersions(c *gin.Context) {
	id := c.Param("id")

	versions, err := data.ListVolumeVersions(c.Request.Context(), id)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, versions)
}

// Get one version of a volume.
//
//	@Summary		Get a volume version
//	@Description	One version's full field snapshot, regardless of whether it's current.
//	@Tags			volumes
//	@Produce		json
//	@Param			id		path		string	true	"Volume ID"
//	@Param			version	path		int		true	"Version number"
//	@Success		200		{object}	vo.VolumeVersionVO
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/versions/{version} [get]
func getVolumeVersion(c *gin.Context) {
	id := c.Param("id")
	version, ok := versionParam(c)
	if !ok {
		return
	}

	result, err := data.GetVolumeVersion(c.Request.Context(), id, version)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if result == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Accept a submitted version, in full or in part.
//
//	@Summary		Accept a submitted volume version
//	@Description	editor/admin only. Omit "fields" to accept every changed field.
//	@Tags			volumes
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Volume ID"
//	@Param			version	path		int						true	"Submitted version number"
//	@Param			request	body		acceptVersionRequest	false	"Fields to accept"
//	@Success		200		{object}	reviewVersionResponse
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/versions/{version}/accept [post]
func acceptVolumeVersion(c *gin.Context, assetsClient *assets.Client) {
	id := c.Param("id")
	version, ok := versionParam(c)
	if !ok {
		return
	}

	var req acceptVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	var selectedFields []string
	if req.Fields != nil {
		selectedFields = *req.Fields
	}

	submitted, err := data.GetVolumeVersion(c.Request.Context(), id, version)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if submitted == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	// Staged edit-session assets ride along with the submission as a whole rather than being a
	// selectable field - see design.md's "Staged edit-session assets ride on the submitted
	// version as separate staged fields". Promoting them here (rather than in AcceptVolumeVersion
	// itself, which stays Mongo-only) mirrors finalizeVolumeSession's editor/admin promote-then-
	// patch pattern.
	var liveCoverAssetId *string
	var liveSampleAssetIds []string
	if submitted.StagedCoverAssetId != nil {
		if err := assetsClient.Promote(c.Request.Context(), "cover-staged", *submitted.StagedCoverAssetId, "cover", id); err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "cover_promote_failed", Message: err.Error()})
			return
		}
		coverID := id
		liveCoverAssetId = &coverID
	}
	if len(submitted.StagedSampleAssetIds) > 0 {
		liveSampleAssetIds = make([]string, len(submitted.StagedSampleAssetIds))
		for i, stagedID := range submitted.StagedSampleAssetIds {
			liveID := fmt.Sprintf("%s-%d", id, i)
			if err := assetsClient.Promote(c.Request.Context(), "sample-staged", stagedID, "sample", liveID); err != nil {
				sentry.CaptureException(err)
				c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "sample_promote_failed", Message: err.Error()})
				return
			}
			liveSampleAssetIds[i] = liveID
		}
	}

	accepted, conflicts, err := data.AcceptVolumeVersion(c.Request.Context(), id, version, selectedFields, authz.Subject(c), nil, liveCoverAssetId, liveSampleAssetIds)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "accept_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, reviewVersionResponse{
		Version:   accepted.Version,
		State:     string(accepted.State),
		Conflicts: conflicts,
	})
}

// Reject a submitted version in full.
//
//	@Summary		Reject a submitted volume version
//	@Description	editor/admin only.
//	@Tags			volumes
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Volume ID"
//	@Param			version	path		int						true	"Submitted version number"
//	@Param			request	body		rejectVersionRequest	false	"Optional review note"
//	@Success		200		{object}	reviewVersionResponse
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/versions/{version}/reject [post]
func rejectVolumeVersion(c *gin.Context) {
	id := c.Param("id")
	version, ok := versionParam(c)
	if !ok {
		return
	}

	var req rejectVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if err := data.RejectVolumeVersion(c.Request.Context(), id, version, authz.Subject(c), note); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "reject_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, reviewVersionResponse{Version: version, State: "rejected"})
}

// Retract a pending submitted version.
//
//	@Summary		Retract a submitted volume version
//	@Description	submitter only, and only their own submissions.
//	@Tags			volumes
//	@Produce		json
//	@Param			id		path		string	true	"Volume ID"
//	@Param			version	path		int		true	"Submitted version number"
//	@Success		200		{object}	reviewVersionResponse
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/versions/{version}/retract [post]
func retractVolumeVersion(c *gin.Context) {
	id := c.Param("id")
	version, ok := versionParam(c)
	if !ok {
		return
	}

	retracted, err := data.RetractVolumeVersion(c.Request.Context(), id, version, authz.Subject(c))
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "retract_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, reviewVersionResponse{Version: retracted.Version, State: string(retracted.State)})
}

// pullBackVersionResponse is the response for a successful pull-back.
type pullBackVersionResponse struct {
	RecordID string `json:"recordId"`
	Status   string `json:"status"`
}

// Pull a pending submitted version back into a fresh edit session, retracting it in the process.
//
//	@Summary		Pull a submitted volume version back into an edit session
//	@Description	submitter only, and only their own submissions. Fails with 409 if the caller already has an in-flight session for this record type.
//	@Tags			volumes
//	@Produce		json
//	@Param			id		path		string	true	"Volume ID"
//	@Param			version	path		int		true	"Submitted version number"
//	@Success		200		{object}	pullBackVersionResponse
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Failure		409		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/versions/{version}/pull-back [post]
func pullBackVolumeVersion(c *gin.Context, editSessions *editsession.Store) {
	id := c.Param("id")
	version, ok := versionParam(c)
	if !ok {
		return
	}
	userID := authz.Subject(c)

	submitted, err := data.GetVolumeVersion(c.Request.Context(), id, version)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if submitted == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
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

	var stagedCoverAssetId string
	if submitted.StagedCoverAssetId != nil {
		stagedCoverAssetId = *submitted.StagedCoverAssetId
	}
	now := time.Now()
	session := editsession.Session{
		RecordID: submitted.RecordID,
		Fields: map[string]any{
			"title":       submitted.Title,
			"description": submitted.Description,
			"notes":       submitted.Notes,
		},
		StagedCoverAssetId: stagedCoverAssetId,
		SampleAssetIds:     submitted.StagedSampleAssetIds,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := editSessions.Set(c.Request.Context(), userID, recordTypeVolume, session); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "session_create_failed", Message: err.Error()})
		return
	}

	retracted, err := data.RetractVolumeVersion(c.Request.Context(), id, version, userID)
	if err != nil {
		// Compensate: the session was created but the source version couldn't be retracted -
		// "creates the session and retracts the source, or does neither", same pattern as
		// pullBackVolumeProposedChange. Roll the session back out rather than leaving one
		// without the other.
		if delErr := editSessions.Delete(c.Request.Context(), userID, recordTypeVolume); delErr != nil {
			sentry.CaptureException(delErr)
		}
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "pull_back_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, pullBackVersionResponse{RecordID: retracted.RecordID, Status: string(retracted.State)})
}

// Roll a volume back (or forward) to an arbitrary existing version.
//
//	@Summary		Set a volume's current version
//	@Description	admin only. Marks the given version live and archives whichever version was previously current.
//	@Tags			volumes
//	@Produce		json
//	@Param			id		path		string	true	"Volume ID"
//	@Param			version	path		int		true	"Version number to make current"
//	@Success		200		{object}	vo.VolumeVersionVO
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		404		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/volumes/{id}/versions/{version}/current [post]
func setCurrentVolumeVersion(c *gin.Context) {
	id := c.Param("id")
	version, ok := versionParam(c)
	if !ok {
		return
	}

	result, err := data.SetCurrentVolumeVersion(c.Request.Context(), id, version)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "rollback_failed", Message: err.Error()})
		return
	}
	if result == nil {
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	c.JSON(http.StatusOK, result)
}
