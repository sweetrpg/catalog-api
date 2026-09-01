package server

import (
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-api/internal/events"
)

func SetupHandlers(g *gin.Engine, cache persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client, assetsClient *assets.Client, editSessions *editsession.Store, eventPublisher *events.Publisher) {
	setupContributionHandlers(g, cache, ttls)
	setupLicenseHandlers(g, cache, ttls, authzClient, eventPublisher)
	setupPersonHandlers(g, cache, ttls, authzClient, eventPublisher)
	setupPublisherHandlers(g, cache, ttls, authzClient, eventPublisher)
	setupReviewHandlers(g, cache, ttls)
	setupStatsHandlers(g, cache, ttls)
	setupStudioHandlers(g, cache, ttls, authzClient, eventPublisher)
	setupSystemHandlers(g, cache, ttls, eventPublisher)
	setupVolumeHandlers(g, cache, ttls, authzClient, assetsClient, editSessions, eventPublisher)
	setupVocabularyHandlers(g, authzClient)
	setupStagedAssetHandlers(g)
	setupSubmissionCapHandlers(g, authzClient)
	setupStatusHandlers(g)
}
