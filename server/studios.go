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

func setupStudioHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up studio endpoint handlers...")
	g.GET("/studios", cache.CachePage(store, time.Hour, listStudios))
	g.GET("/studios/:id", cache.CachePage(store, time.Hour, getStudio))
	// g.GET("/studios/:id/studios", cache.CachePage(store, time.Hour, getStudioStudios))
}

func listStudios(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "list-studios")
	defer span.End()

	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))
	studios, err := database.Query[models.Studio]("studios", bson.D{}, "title", start, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, studios); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getStudio(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "get-studios")
	defer span.End()

	id := c.Param("id")
	studio, err := database.Get[models.Studio]("studios", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if studio == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, studio); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
