package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/submissioncap"
	"github.com/sweetrpg/common.go/logging"
)

func setupSubmissionCapHandlers(g *gin.Engine, authzClient *authz.Client) {
	logging.Logger.Info("Setting up submission cap endpoint handlers...")

	adminOnly := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin)
	g.PUT("/users/:userId/submission-cap", adminOnly, setSubmissionCapOverride)
}

// setSubmissionCapRequest's Cap is a pointer so an absent/null value (clear the override) is
// distinguishable from an explicit 0 (a real cap of zero).
type setSubmissionCapRequest struct {
	Cap *int `json:"cap"`
}

type submissionCapResponse struct {
	UserID string `json:"userId"`
	Cap    int    `json:"cap"`
}

// Set or clear a user's unapproved-submission cap override.
//
//	@Summary		Set a user's submission cap override
//	@Description	admin only. Omit "cap" (or pass null) to clear the override, falling the user back to the platform default.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			userId	path		string					true	"User ID"
//	@Param			request	body		setSubmissionCapRequest	true	"Cap to set, or null to clear"
//	@Success		200		{object}	submissionCapResponse
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/users/{userId}/submission-cap [put]
func setSubmissionCapOverride(c *gin.Context) {
	userID := c.Param("userId")

	var req setSubmissionCapRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}

	if req.Cap == nil {
		if err := submissioncap.ClearOverride(c.Request.Context(), userID); err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "clear_failed", Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, submissionCapResponse{UserID: userID, Cap: submissioncap.Default()})
		return
	}

	if err := submissioncap.SetOverride(c.Request.Context(), userID, *req.Cap); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "set_failed", Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, submissionCapResponse{UserID: userID, Cap: *req.Cap})
}
