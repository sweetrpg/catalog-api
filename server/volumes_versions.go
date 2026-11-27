package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/common.go/logging"
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
	logging.Logger.Debug("listVolumeVersions: enter", "id", id)

	versions, err := data.ListVolumeVersions(c.Request.Context(), id, versionReader(c))
	if err != nil {
		logging.Logger.Error("listVolumeVersions: query failed", "id", id, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	logging.Logger.Debug("listVolumeVersions: exit", "id", id, "count", len(versions))

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
	logging.Logger.Debug("getVolumeVersion: enter", "id", id, "version", version)

	result, err := data.GetVolumeVersion(c.Request.Context(), id, version, versionReader(c))
	if err != nil {
		logging.Logger.Error("getVolumeVersion: query failed", "id", id, "version", version, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if result == nil {
		logging.Logger.Debug("getVolumeVersion: exit", "id", id, "outcome", "not_found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}
	logging.Logger.Debug("getVolumeVersion: exit", "id", id, "version", version, "outcome", "ok")

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
	logging.Logger.Debug("acceptVolumeVersion: enter", "id", id, "version", version)

	var req acceptVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		logging.Logger.Debug("acceptVolumeVersion: exit", "id", id, "outcome", "bad_request")
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	var selectedFields []string
	if req.Fields != nil {
		selectedFields = *req.Fields
	}

	submitted, err := data.GetVolumeVersion(c.Request.Context(), id, version, internalReader)
	if err != nil {
		logging.Logger.Error("acceptVolumeVersion: version lookup failed", "id", id, "version", version, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if submitted == nil {
		logging.Logger.Debug("acceptVolumeVersion: exit", "id", id, "outcome", "not_found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	// Checked before any file is copied: copying overwrites the live images.
	if string(submitted.State) != string(catalogmodels.VersionStateSubmitted) {
		logging.Logger.Warn("acceptVolumeVersion: not pending", "id", id, "version", version, "state", submitted.State)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "accept_failed", Message: "version is not awaiting review"})
		return
	}

	// Staged edit-session assets ride along with the submission as a whole rather than being a
	// selectable field - see design.md's "Staged edit-session assets ride on the submitted
	// version as separate staged fields". Moving them here keeps AcceptVolumeVersion Mongo-only.
	var stagedCoverID string
	if submitted.StagedCoverAssetId != nil {
		stagedCoverID = *submitted.StagedCoverAssetId
	}
	staged := newPromotion(id, stagedCoverID, submitted.StagedSampleAssetIds)
	if code, err := staged.copy(c.Request.Context(), assetsClient, authz.Token(c)); err != nil {
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: code, Message: err.Error()})
		return
	}

	accepted, conflicts, err := data.AcceptVolumeVersion(c.Request.Context(), id, version, selectedFields, authz.Viewer(c), nil, staged.coverID, staged.sampleIDs)
	if err != nil {
		logging.Logger.Error("acceptVolumeVersion: accept failed", "id", id, "version", version, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "accept_failed", Message: err.Error()})
		return
	}
	staged.deleteStaged(c.Request.Context(), assetsClient, authz.Token(c))

	if volumeEventPublisher != nil {
		result, err := data.GetVolume(c.Request.Context(), id)
		if err == nil && result != nil {
			volumeEventPublisher.PublishEntityUpdated(c.Request.Context(), "volume", id, accepted.Version, map[string]interface{}{
				"title": result.Title,
			})
		}
	}

	logging.Logger.Info("acceptVolumeVersion: exit", "id", id, "version", version, "outcome", "ok", "conflicts", len(conflicts))
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
	logging.Logger.Debug("rejectVolumeVersion: enter", "id", id, "version", version)

	var req rejectVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if err := data.RejectVolumeVersion(c.Request.Context(), id, version, authz.Viewer(c), note); err != nil {
		logging.Logger.Error("rejectVolumeVersion: reject failed", "id", id, "version", version, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "reject_failed", Message: err.Error()})
		return
	}

	logging.Logger.Info("rejectVolumeVersion: exit", "id", id, "version", version, "outcome", "ok")
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
	logging.Logger.Debug("retractVolumeVersion: enter", "id", id, "version", version)

	retracted, err := data.RetractVolumeVersion(c.Request.Context(), id, version, authz.Viewer(c))
	if errors.Is(err, data.ErrVersionNotFound) {
		logging.Logger.Debug("retractVolumeVersion: exit", "id", id, "version", version, "outcome", "not_found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}
	if err != nil {
		logging.Logger.Error("retractVolumeVersion: retract failed", "id", id, "version", version, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "retract_failed", Message: err.Error()})
		return
	}
	logging.Logger.Info("retractVolumeVersion: exit", "id", id, "version", version, "outcome", "ok")

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
	userID := authz.Viewer(c)
	// The shared edit-session store is keyed by the raw IdP subject (catalog-web, the writer,
	// has no canonical users._id). Session reads/writes here use that key; authz.Viewer stays
	// the acting user for the retract below.
	sessionKey := authz.Subject(c)
	logging.Logger.Debug("pullBackVolumeVersion: enter", "id", id, "version", version, "userId", userID)

	submitted, err := data.GetVolumeVersion(c.Request.Context(), id, version, roleReader(c))
	if err != nil {
		logging.Logger.Error("pullBackVolumeVersion: version lookup failed", "id", id, "version", version, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}
	if submitted == nil {
		logging.Logger.Debug("pullBackVolumeVersion: exit", "id", id, "outcome", "not_found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}

	existingSession, err := editSessions.Get(c.Request.Context(), sessionKey, recordTypeVolume)
	if err != nil {
		logging.Logger.Error("pullBackVolumeVersion: session lookup failed", "userId", userID, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "session_lookup_failed", Message: err.Error()})
		return
	}
	if existingSession != nil {
		logging.Logger.Warn("pullBackVolumeVersion: session already exists", "userId", userID)
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
	if err := editSessions.Set(c.Request.Context(), sessionKey, recordTypeVolume, session); err != nil {
		logging.Logger.Error("pullBackVolumeVersion: session create failed", "userId", userID, "error", err)
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
		if delErr := editSessions.Delete(c.Request.Context(), sessionKey, recordTypeVolume); delErr != nil {
			sentry.CaptureException(delErr)
		}
		if errors.Is(err, data.ErrVersionNotFound) {
			logging.Logger.Debug("pullBackVolumeVersion: exit", "id", id, "outcome", "not_found")
			c.JSON(http.StatusNotFound, apiv.ErrorVO{})
			return
		}
		logging.Logger.Error("pullBackVolumeVersion: retract failed after session creation, session rolled back", "id", id, "version", version, "userId", userID, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "pull_back_failed", Message: err.Error()})
		return
	}

	logging.Logger.Info("pullBackVolumeVersion: exit", "id", id, "version", version, "userId", userID, "outcome", "ok")
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
	logging.Logger.Debug("setCurrentVolumeVersion: enter", "id", id, "version", version)

	result, err := data.SetCurrentVolumeVersion(c.Request.Context(), id, version, authz.Viewer(c))
	if err != nil {
		logging.Logger.Error("setCurrentVolumeVersion: rollback failed", "id", id, "version", version, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "rollback_failed", Message: err.Error()})
		return
	}
	if result == nil {
		logging.Logger.Debug("setCurrentVolumeVersion: exit", "id", id, "outcome", "not_found")
		c.JSON(http.StatusNotFound, apiv.ErrorVO{})
		return
	}
	publishCurrentVolume(c, id)
	logging.Logger.Info("setCurrentVolumeVersion: exit", "id", id, "version", version, "outcome", "ok")

	c.JSON(http.StatusOK, result)
}
