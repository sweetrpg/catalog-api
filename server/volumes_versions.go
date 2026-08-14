package server

import (
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
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
func acceptVolumeVersion(c *gin.Context) {
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

	accepted, conflicts, err := data.AcceptVolumeVersion(c.Request.Context(), id, version, selectedFields, authz.Subject(c), nil)
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
