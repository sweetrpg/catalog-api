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
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupPublisherHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up publisher endpoint handlers...")
	g.GET("/publishers", cache.CachePage(store, time.Hour, listPublishers))
	g.GET("/publishers/:id", cache.CachePage(store, time.Hour, getPublisher))
	// g.GET("/publishers/:id/publishers", cache.CachePage(store, time.Hour, getPublisherPublishers))
}

func listPublishers(c *gin.Context) {
	listParams := util.GetListQueryParams(c)

	_, span := tracer.Start(c.Request.Context(), "query-database")
	publishers, err := database.Query[models.Publisher]("publishers", bson.D{}, "name", listParams.Start, listParams.Limit)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, publishers); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getPublisher(c *gin.Context) {
	id := c.Param("id")

	_, span := tracer.Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id)))
	publisher, err := database.Get[models.Publisher]("publishers", id)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if publisher == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, publisher); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
