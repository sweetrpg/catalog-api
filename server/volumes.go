package server

import (
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-api/data"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/util"
	options "go.jtlabs.io/query"
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
	opt, _ := options.FromQuerystring(c.Request.URL.RawQuery)

	span := util.BuildSpanWithOptions(c.Request.Context(), "volumes", "list-volumes", opt)
	vos, err := data.GetVolumes(c.Request.Context(), bson.D{}, opt)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, vos); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getVolume(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("volumes").Start(c.Request.Context(), "get-volume", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetVolume(c.Request.Context(), id)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if vo == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, vo); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
