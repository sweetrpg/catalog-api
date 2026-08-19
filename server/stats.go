package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
)

func setupStatsHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config) {
	logging.Logger.Info("Setting up stats endpoint handlers...")
	ttl := ttls.TTL("stats")
	g.GET("/stats", cache.CachePage(store, ttl, getCatalogStats))
}

// catalogStatsResponse is a plain JSON object, not a JSON:API resource - this is a singleton
// aggregate over the whole catalog, not a record with an id, so the usual jsonapi.MarshalPayload
// convention every other route here uses doesn't fit.
type catalogStatsResponse struct {
	VolumeCount int     `json:"volume_count"`
	LastUpdated *string `json:"last_updated"`
}

// Get catalog-wide stats.
//
//	@Summary		Get catalog stats
//	@Description	Returns catalog-wide summary stats (total volume count, most recent update) in a single call, rather than requiring a caller to page through every volume itself.
//	@Tags			stats
//	@Produce		json
//	@Success		200	{object}	catalogStatsResponse
//	@Failure		500	{object}	interface{}
//	@Router			/stats [get]
func getCatalogStats(c *gin.Context) {
	stats, err := data.GetCatalogStats(c.Request.Context())
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := catalogStatsResponse{VolumeCount: stats.VolumeCount}
	if stats.LastUpdated != nil {
		formatted := stats.LastUpdated.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.LastUpdated = &formatted
	}

	c.JSON(http.StatusOK, resp)
}
