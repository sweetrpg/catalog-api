package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Systems are read-only here: gamesystems-api is the system of record (see
// catalog-data.go's GameSystemsClient), so there's no create/update/delete path to wire up -
// only list/get, resolved live against gamesystems-api rather than a local Mongo collection.
func setupSystemHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config) {
	logging.Logger.Info("Setting up system endpoint handlers...")
	ttl := ttls.TTL("systems")
	g.GET("/systems", cache.CachePage(store, ttl, listSystems))
	g.GET("/systems/search", cache.CachePage(store, ttl, searchSystems))
	g.GET("/systems/:id", cache.CachePage(store, ttl, getSystem))
}

// List systems.
//
//	@Summary		List systems
//	@Description	Lists the systems in the database.
//	@Tags			systems
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/systems [get]
func listSystems(c *gin.Context) {
	_, span := otel.Tracer("systems").Start(c.Request.Context(), "list-systems")
	vos, err := data.QuerySystems(c.Request.Context())
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

// Search systems.
//
//	@Summary		Search systems
//	@Description	Finds game systems whose name contains the query string - backs autocomplete/picker inputs.
//	@Tags			systems
//	@Produce		json
//	@Param			q		query		string			true	"Search query"
//	@Success		200		{object}	interface{}
//	@Failure		400		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/systems/search [get]
func searchSystems(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}

	_, span := otel.Tracer("systems").Start(c.Request.Context(), "search-systems", oteltrace.WithAttributes(attribute.String("q", q)))
	vos, err := data.SearchSystems(c.Request.Context(), q)
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

// Get a system.
//
//	@Summary		Get a system
//	@Description	Get the details of a system from the database.
//	@Tags			systems
//	@Produce		json
//	@Param			id		path		string			true	"System ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/systems/{id} [get]
func getSystem(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("systems").Start(c.Request.Context(), "get-system", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetSystem(c.Request.Context(), id)
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
