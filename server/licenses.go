package server

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("licenses")

func setupLicenseHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up license endpoint handlers...")
	g.GET("/licenses", cache.CachePage(store, time.Hour, listLicenses))
	g.GET("/licenses/:id", cache.CachePage(store, time.Hour, getLicense))
	g.GET("/licenses/:id/volumes", cache.CachePage(store, time.Hour, getLicenseVolumes))
}

func listLicenses(c *gin.Context) {
	// _, span := tracing.Tracer.Start(c, "list-licenses")
	// defer span.End()

	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))

	_, span := tracer.Start(c.Request.Context(), "query-licenses")
	licenses, err := database.Query[models.License]("licenses", bson.D{}, "title", start, limit)
	span.End()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, licenses); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getLicenseVolumes(c *gin.Context) {
	// _, span := tracing.Tracer.Start(c, "get-license-volumes")
	// defer span.End()

	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
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

	_, span := tracer.Start(c.Request.Context(), "get-license-volumes", oteltrace.WithAttributes(attribute.String("id", id.String())))
	volumes, err := database.Query[models.Volume]("volumes", filter, "title", start, limit) // TODO:
	span.End()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, volumes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getLicense(c *gin.Context) {
	// _, span := tracing.Tracer.Start(c, "get-licenses")
	// defer span.End()

	id := c.Param("id")

	_, span := tracer.Start(c.Request.Context(), "get-licenses", oteltrace.WithAttributes(attribute.String("id", id)))
	license, err := database.Get[models.License]("licenses", id)
	span.End()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if license == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, license); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
