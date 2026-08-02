package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/api-core.go/tracing"
	apiutil "github.com/sweetrpg/api-core.go/util"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupStudioHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config) {
	logging.Logger.Info("Setting up studio endpoint handlers...")
	ttl := ttls.TTL("studios")
	g.GET("/studios", cache.CachePage(store, ttl, listStudios))
	g.GET("/studios/:id", cache.CachePage(store, ttl, getStudio))
	// g.GET("/studios/:id/studios", cache.CachePage(store, ttl, getStudioStudios))
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
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "studios", "list-studios", params)
	vos, err := data.QueryStudios(c.Request.Context(), params)
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
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, vo); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
