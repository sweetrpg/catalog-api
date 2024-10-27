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

func setupPersonHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up person endpoint handlers...")
	g.GET("/persons", cache.CachePage(store, time.Hour, listPersons))
	g.GET("/persons/:id", cache.CachePage(store, time.Hour, getPerson))
	// g.GET("/persons/:id/persons", cache.CachePage(store, time.Hour, getPersonPersons))
}

func listPersons(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "list-persons")
	defer span.End()

	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))
	persons, err := database.Query[models.Person]("persons", bson.D{}, "name", start, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, persons); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getPerson(c *gin.Context) {
	_, span := tracing.Tracer.Start(c, "get-persons")
	defer span.End()

	id := c.Param("id")
	person, err := database.Get[models.Person]("persons", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if person == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, person); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
