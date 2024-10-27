package server

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/models"
	"github.com/sweetrpg/catalog-api/tracing"
	"go.mongodb.org/mongo-driver/bson"
)

func setupSystemHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up system endpoint handlers...")
	g.GET("/systems", cache.CachePage(store, time.Hour, listSystems))
	g.GET("/systems/:id", cache.CachePage(store, time.Hour, getSystem))
	// g.GET("/systems/:id/systems", cache.CachePage(store, time.Hour, getSystemSystems))
}

func listSystems(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "list-systems")
	defer span.End()

	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))
	systems, err := database.Query[models.System]("systems", bson.D{}, "title", start, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, systems); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getSystem(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "get-systems")
	defer span.End()

	id := c.Param("id")
	system, err := database.Get[models.System]("systems", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if system == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, system); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
