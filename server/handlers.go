package server

import (
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/editsession"
)

func SetupHandlers(g *gin.Engine, cache persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client, assetsClient *assets.Client, editSessions *editsession.Store) {
	setupContributionHandlers(g, cache, ttls)
	setupLicenseHandlers(g, cache, ttls, authzClient)
	setupPersonHandlers(g, cache, ttls, authzClient)
	setupPublisherHandlers(g, cache, ttls, authzClient)
	setupReviewHandlers(g, cache, ttls)
	setupStudioHandlers(g, cache, ttls, authzClient)
	setupSystemHandlers(g, cache, ttls, authzClient)
	setupVolumeHandlers(g, cache, ttls, authzClient, assetsClient, editSessions)
	setupVocabularyHandlers(g, authzClient)
	setupSubmissionCapHandlers(g, authzClient)
	setupStatusHandlers(g)
}
