package server

import (
	"fmt"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/models"
	"go.mongodb.org/mongo-driver/bson"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"time"
)

func setupLicenseHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up license endpoint handlers...")
	g.GET("/licenses", cache.CachePage(store, time.Hour, listLicenses))
	g.GET("/licenses/:id", cache.CachePage(store, time.Hour, getLicense))
}

func listLicenses(c *gin.Context) {
	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))
	licenses, err := database.Query[models.License]("licenses", bson.D{}, "title", start, limit)
	logging.Logger.Debug(fmt.Sprintf("licenses=%v", reflect.TypeOf(licenses)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, licenses); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getLicense(c *gin.Context) {
	id := c.Param("id")
	license, err := database.Get[models.License]("licenses", id)
	logging.Logger.Debug(fmt.Sprintf("license=%v", reflect.TypeOf(license)))
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
