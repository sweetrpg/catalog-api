package server

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"net/http"
	"os"
	"time"
)

func setupStatusHandlers(g *gin.Engine) {
	logging.Logger.Info("Setting up status endpoint handlers...")
	g.GET("/status/health", healthHandler)
	g.GET("/status/ping", pingHandler)
}

func healthHandler(c *gin.Context) {
	collections, _ := database.Db.ListCollectionNames(context.TODO(), bson.D{})
	start := time.Now()
	database.Db.Client().Ping(context.TODO(), readpref.Primary())
	duration := time.Since(start)
	c.JSON(http.StatusOK, gin.H{
		"database": database.Db.Name(),
		"ping":     fmt.Sprintf("%dms", duration.Milliseconds()),
		// "user": dbUser,
		"collections": collections,
	})
}

func pingHandler(c *gin.Context) {
	hostname, _ := os.Hostname()
	c.JSON(http.StatusOK, gin.H{
		"pong": gin.H{
			"date":     time.Now(),
			"hostname": hostname,
		},
	})
}
