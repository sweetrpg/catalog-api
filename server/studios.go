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

func setupStudioHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up studio endpoint handlers...")
	g.GET("/studios", cache.CachePage(store, time.Hour, listStudios))
	g.GET("/studios/:id", cache.CachePage(store, time.Hour, getStudio))
	// g.GET("/studios/:id/studios", cache.CachePage(store, time.Hour, getStudioStudios))
}

func listStudios(c *gin.Context) {
	listParams := util.GetListQueryParams(c)

	_, span := otel.Tracer("studios").Start(c.Request.Context(), "query-database")
	studios, err := database.Query[models.Studio]("studios", bson.D{}, "name", listParams.Start, listParams.Limit)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, studios); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getStudio(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("studios").Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id)))
	studio, err := database.Get[models.Studio]("studios", id)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if studio == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, studio); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
