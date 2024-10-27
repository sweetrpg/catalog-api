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
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupContributionHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up contribution endpoint handlers...")
	g.GET("/contributions", cache.CachePage(store, time.Hour, listContributions))
	g.GET("/contributions/:id", cache.CachePage(store, time.Hour, getContribution))
	// g.GET("/contributions/:id/contributions", cache.CachePage(store, time.Hour, getContributionContributions))
}

func listContributions(c *gin.Context) {
	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))

	_, span := tracer.Start(c.Request.Context(), "query-database")
	contributions, err := database.Query[models.Contribution]("contributions", bson.D{}, "_id", start, limit)
	span.End()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, contributions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getContribution(c *gin.Context) {
	id := c.Param("id")

	_, span := tracer.Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id)))
	contribution, err := database.Get[models.Contribution]("contributions", id)
	span.End()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if contribution == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, contribution); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
