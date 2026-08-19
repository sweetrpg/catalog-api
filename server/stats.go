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
// convention every other route here uses doesn't fit. Nothing besides the landing page consumes
// `/stats` today, so this replaces the old volume-only shape (volume_count/last_updated)
// outright rather than keeping it alongside the new per-type fields - see design.md's Migration
// Plan.
type catalogStatsResponse struct {
	Volumes    typeStatsVO `json:"volumes"`
	Publishers typeStatsVO `json:"publishers"`
	Studios    typeStatsVO `json:"studios"`
	Persons    typeStatsVO `json:"persons"`
	Licenses   typeStatsVO `json:"licenses"`
	Systems    typeStatsVO `json:"systems"`
}

// typeStatsVO is one entity type's landing-page-summary card (catalog-landing-page-summary) -
// MostRecent is omitted (nil), not an empty object, when the type has zero records, per the
// spec's "degrades gracefully for an empty entity type" requirement.
type typeStatsVO struct {
	Count       int                    `json:"count"`
	LastUpdated *string                `json:"last_updated"`
	MostRecent  *typeStatsMostRecentVO `json:"most_recent"`
}

type typeStatsMostRecentVO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toTypeStatsVO(stats *data.TypeStats) typeStatsVO {
	vo := typeStatsVO{Count: stats.Count}
	if stats.LastUpdated != nil {
		formatted := stats.LastUpdated.UTC().Format("2006-01-02T15:04:05Z07:00")
		vo.LastUpdated = &formatted
	}
	if stats.MostRecentID != "" {
		vo.MostRecent = &typeStatsMostRecentVO{ID: stats.MostRecentID, Name: stats.MostRecentName}
	}
	return vo
}

// Get catalog-wide stats.
//
//	@Summary		Get catalog stats
//	@Description	Returns catalog-wide summary stats (per-entity-type count, most recent update, and most recently added record) in a single call, rather than requiring a caller to page through every entity type itself.
//	@Tags			stats
//	@Produce		json
//	@Success		200	{object}	catalogStatsResponse
//	@Failure		500	{object}	interface{}
//	@Router			/stats [get]
func getCatalogStats(c *gin.Context) {
	volumeStats, err := data.GetVolumeTypeStats(c.Request.Context())
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	publisherStats, err := data.GetPublisherStats(c.Request.Context())
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	studioStats, err := data.GetStudioStats(c.Request.Context())
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	personStats, err := data.GetPersonStats(c.Request.Context())
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	licenseStats, err := data.GetLicenseStats(c.Request.Context())
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	systemStats, err := data.GetSystemStats(c.Request.Context())
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := catalogStatsResponse{
		Volumes:    toTypeStatsVO(volumeStats),
		Publishers: toTypeStatsVO(publisherStats),
		Studios:    toTypeStatsVO(studioStats),
		Persons:    toTypeStatsVO(personStats),
		Licenses:   toTypeStatsVO(licenseStats),
		Systems:    toTypeStatsVO(systemStats),
	}

	c.JSON(http.StatusOK, resp)
}
