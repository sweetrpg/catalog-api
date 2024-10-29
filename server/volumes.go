package server

import (
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/models"
	"github.com/sweetrpg/catalog-api/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/otel"
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
	listParams := util.GetListQueryParams(c)

	_, span := otel.Tracer("volumes").Start(c.Request.Context(), "query-database")
	volumes, err := database.Query[models.Volume]("volumes", bson.D{}, "title", listParams.Start, listParams.Limit)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, volumes); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getVolume(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("volumes").Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id)))
	volume, err := database.Get[models.Volume]("volumes", id)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if volume == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, volume); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
