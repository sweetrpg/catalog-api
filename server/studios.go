package server

import (
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/api-core/tracing"
	"github.com/sweetrpg/catalog-data/data"
	"github.com/sweetrpg/common/logging"
	options "go.jtlabs.io/query"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupStudioHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up studio endpoint handlers...")
	g.GET("/studios", cache.CachePage(store, time.Hour, listStudios))
	g.GET("/studios/:id", cache.CachePage(store, time.Hour, getStudio))
	// g.GET("/studios/:id/studios", cache.CachePage(store, time.Hour, getStudioStudios))
}

// List studios.
//
//	@Summary		List studios
//	@Description	Lists the studios in the database.
//	@Tags			studios
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/studios [get]
func listStudios(c *gin.Context) {
	opt, _ := options.FromQuerystring(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithOptions(c.Request.Context(), "studios", "list-studios", opt)
	vos, err := data.GetStudios(c.Request.Context(), bson.D{}, opt)
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

// Get a studio.
//
//	@Summary		Get a studio
//	@Description	Get the details of a studio from the database.
//	@Tags			studios
//	@Produce		json
//	@Param			id		path		string			true	"Studio ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/studios/{id} [get]
func getStudio(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("studios").Start(c.Request.Context(), "get-studio", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetStudio(c.Request.Context(), id)
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
