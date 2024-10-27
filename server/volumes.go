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

func setupVolumeHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up volume endpoint handlers...")
	g.GET("/volumes", cache.CachePage(store, time.Hour, listVolumes))
	g.GET("/volumes/:id", cache.CachePage(store, time.Hour, getVolume))
	// g.GET("/volumes/:id/volumes", cache.CachePage(store, time.Hour, getVolumeVolumes))
}

func listVolumes(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "list-volumes")
	defer span.End()

	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))
	volumes, err := database.Query[models.Volume]("volumes", bson.D{}, "title", start, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, volumes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getVolume(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "get-volumes")
	defer span.End()

	id := c.Param("id")
	volume, err := database.Get[models.Volume]("volumes", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if volume == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, volume); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
