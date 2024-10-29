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

func setupSystemHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up system endpoint handlers...")
	g.GET("/systems", cache.CachePage(store, time.Hour, listSystems))
	g.GET("/systems/:id", cache.CachePage(store, time.Hour, getSystem))
	// g.GET("/systems/:id/systems", cache.CachePage(store, time.Hour, getSystemSystems))
}

func listSystems(c *gin.Context) {
	listParams := util.GetListQueryParams(c)

	_, span := otel.Tracer("systems").Start(c.Request.Context(), "query-database")
	systems, err := database.Query[models.System]("systems", bson.D{}, "game_system", listParams.Start, listParams.Limit)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, systems); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getSystem(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("systems").Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id)))
	system, err := database.Get[models.System]("systems", id)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if system == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, system); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
