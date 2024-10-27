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
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupVolumeHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up volume endpoint handlers...")
	g.GET("/volumes", cache.CachePage(store, time.Hour, listVolumes))
	g.GET("/volumes/:id", cache.CachePage(store, time.Hour, getVolume))
	// g.GET("/volumes/:id/volumes", cache.CachePage(store, time.Hour, getVolumeVolumes))
}

func listVolumes(c *gin.Context) {
	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))

	_, span := tracer.Start(c.Request.Context(), "query-database")
	volumes, err := database.Query[models.Volume]("volumes", bson.D{}, "title", start, limit)
	span.End()
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
	id := c.Param("id")

	_, span := tracer.Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id)))
	volume, err := database.Get[models.Volume]("volumes", id)
	span.End()
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
