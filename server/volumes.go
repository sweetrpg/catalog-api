package server

import (
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/api-core.go/tracing"
	apiutil "github.com/sweetrpg/api-core.go/util"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
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

// List volumes.
//
//	@Summary		List volumes
//	@Description	Lists the volumes in the database.
//	@Tags			volumes
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/volumes [get]
func listVolumes(c *gin.Context) {
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "volumes", "list-volumes", params)
	vos, err := data.GetVolumes(c.Request.Context(), params)
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

// Get a volume.
//
//	@Summary		Get a volume
//	@Description	Get the details of a volume from the database.
//	@Tags			volumes
//	@Produce		json
//	@Param			id		path		string			true	"Volume ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/volumes/{id} [get]
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
