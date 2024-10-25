package server

import (
	// "context"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	// "github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"

	// "github.com/sweetrpg/catalog-api/models"
	"go.mongodb.org/mongo-driver/bson"
	// "go.mongodb.org/mongo-driver/mongo"
)

func setupLicenseHandlers(g *gin.Engine) {
	logging.Logger.Info("Setting up license handlers...")
	g.GET("/licenses", listLicenses)
	g.GET("/licenses/:id", getLicense)
}

func listLicenses(c *gin.Context) {
	start, _ := strconv.Atoi(c.Query("start"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Max(1.0, float64(limit)))
	results, err := database.Query("licenses", bson.D{}, "title", start, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logging.Logger.Debug("query results", "result", results)
	// var licenses []models.License
	for _, r := range results {
		logging.Logger.Debug(fmt.Sprintf("r=%v", r))
		// var license models.License
		// err = bson.Unmarshal(r, &license)
	}
	// err = bson.Unmarshal(result, &licenses)

	c.JSON(http.StatusOK, results) // TODO: JSON-API
}

func getLicense(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}
