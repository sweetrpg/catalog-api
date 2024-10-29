package server

import (
	"math"
	"net/http"
	"strconv"
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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupLicenseHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up license endpoint handlers...")
	g.GET("/licenses", cache.CachePage(store, time.Hour, listLicenses))
	g.GET("/licenses/:id", cache.CachePage(store, time.Hour, getLicense))
	g.GET("/licenses/:id/volumes", cache.CachePage(store, time.Hour, getLicenseVolumes))
}

func listLicenses(c *gin.Context) {
	listParams := util.GetListQueryParams(c)

	_, span := otel.Tracer("licenses").Start(c.Request.Context(), "query-database")
	licenses, err := database.Query[models.License]("licenses", bson.D{}, "title", listParams.Start, listParams.Limit)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, licenses); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getLicenseVolumes(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))

	filter := bson.D{
		{"license_ids",
			bson.D{{"$in", id}},
		},
	}

	_, span := otel.Tracer("licenses").Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id.String())))
	volumes, err := database.Query[models.Volume]("volumes", filter, "title", start, limit)
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
		// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getLicense(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("licenses").Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id)))
	license, err := database.Get[models.License]("licenses", id)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if license == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, license); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
