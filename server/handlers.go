package server

import (
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
)

func SetupHandlers(g *gin.Engine, cache persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client) {
	setupContributionHandlers(g, cache, ttls)
	setupLicenseHandlers(g, cache, ttls)
	setupPersonHandlers(g, cache, ttls)
	setupPublisherHandlers(g, cache, ttls)
	setupReviewHandlers(g, cache, ttls)
	setupStudioHandlers(g, cache, ttls)
	setupSystemHandlers(g, cache, ttls)
	setupVolumeHandlers(g, cache, ttls, authzClient)
	setupVocabularyHandlers(g, authzClient)
	setupStatusHandlers(g)
}
