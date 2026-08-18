package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
)

func setupStagedAssetHandlers(g *gin.Engine) {
	logging.Logger.Info("Setting up staged-asset endpoint handlers...")
	g.GET("/staged-assets/pending", listPendingStagedAssets)
}

type pendingStagedAssetsResponse struct {
	CoverAssetIds  []string `json:"coverAssetIds"`
	SampleAssetIds []string `json:"sampleAssetIds"`
}

// List staged asset ids a pending submission still references.
//
//	@Summary		List staged asset ids referenced by a pending submission
//	@Description	Internal, unauthenticated (matches other GET list endpoints) - called by
//					assets-web's periodic reclaim job so it never deletes a staged cover/sample
//					still needed by a pending volume version.
//	@Tags			staged-assets
//	@Produce		json
//	@Success		200	{object}	pendingStagedAssetsResponse
//	@Failure		500	{object}	apiv.ErrorVO
//	@Router			/staged-assets/pending [get]
func listPendingStagedAssets(c *gin.Context) {
	pending, err := data.ListPendingStagedAssetIds(c.Request.Context())
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, pendingStagedAssetsResponse{
		CoverAssetIds:  pending.CoverAssetIds,
		SampleAssetIds: pending.SampleAssetIds,
	})
}
