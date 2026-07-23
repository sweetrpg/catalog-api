package server

import (
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
)

func SetupHandlers(g *gin.Engine, cache persistence.CacheStore) {
	setupContributionHandlers(g, cache)
	setupLicenseHandlers(g, cache)
	setupPersonHandlers(g, cache)
	setupPublisherHandlers(g, cache)
	setupReviewHandlers(g, cache)
	setupStudioHandlers(g, cache)
	setupSystemHandlers(g, cache)
	setupVolumeHandlers(g, cache)
	setupStatusHandlers(g)
}
