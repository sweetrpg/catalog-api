package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/common/logging"
	"github.com/sweetrpg/db/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.opentelemetry.io/otel"
)

func setupStatusHandlers(g *gin.Engine) {
	logging.Logger.Info("Setting up status endpoint handlers...")
	g.GET("/status/health", healthHandler)
	g.GET("/status/ping", pingHandler)
}

func healthHandler(c *gin.Context) {
	_, span := otel.Tracer("health").Start(c.Request.Context(), "list-collections")
	collections, _ := database.Db.ListCollectionNames(context.TODO(), bson.D{})
	span.End()

	start := time.Now()
	_, span = otel.Tracer("health").Start(c.Request.Context(), "ping-database")
	database.Db.Client().Ping(context.TODO(), readpref.Primary())
	span.End()
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
