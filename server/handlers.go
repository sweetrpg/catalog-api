package server

import (
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
)

func SetupHandlers(g *gin.Engine, cache persistence.CacheStore) {
	setupLicenseHandlers(g, cache)
	setupStatusHandlers(g)
}
