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

func setupPersonHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up person endpoint handlers...")
	g.GET("/persons", cache.CachePage(store, time.Hour, listPersons))
	g.GET("/persons/:id", cache.CachePage(store, time.Hour, getPerson))
	// g.GET("/persons/:id/persons", cache.CachePage(store, time.Hour, getPersonPersons))
}

func listPersons(c *gin.Context) {
	listParams := util.GetListQueryParams(c)

	_, span := tracer.Start(c.Request.Context(), "query-database")
	persons, err := database.Query[models.Person]("persons", bson.D{}, "name", listParams.Start, listParams.Limit)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, persons); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getPerson(c *gin.Context) {
	id := c.Param("id")

	_, span := tracer.Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id)))
	person, err := database.Get[models.Person]("persons", id)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if person == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, person); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
