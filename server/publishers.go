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
	"github.com/sweetrpg/catalog-api/tracing"
	"go.mongodb.org/mongo-driver/bson"
)

func setupPublisherHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up publisher endpoint handlers...")
	g.GET("/publishers", cache.CachePage(store, time.Hour, listPublishers))
	g.GET("/publishers/:id", cache.CachePage(store, time.Hour, getPublisher))
	// g.GET("/publishers/:id/publishers", cache.CachePage(store, time.Hour, getPublisherPublishers))
}

func listPublishers(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "list-publishers")
	defer span.End()

	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))
	publishers, err := database.Query[models.Publisher]("publishers", bson.D{}, "name", start, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, publishers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getPublisher(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "get-publishers")
	defer span.End()

	id := c.Param("id")
	publisher, err := database.Get[models.Publisher]("publishers", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if publisher == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, publisher); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
