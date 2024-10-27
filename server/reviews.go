package server

import (
	"net/http"
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
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupReviewHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up review endpoint handlers...")
	g.GET("/reviews", cache.CachePage(store, time.Hour, listReviews))
	g.GET("/reviews/:id", cache.CachePage(store, time.Hour, getReview))
	// g.GET("/reviews/:id/reviews", cache.CachePage(store, time.Hour, getReviewReviews))
}

func listReviews(c *gin.Context) {
	listParams := util.GetListQueryParams(c)

	_, span := tracer.Start(c.Request.Context(), "query-database")
	reviews, err := database.Query[models.Review]("reviews", bson.D{}, "title", listParams.Start, listParams.Limit)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, reviews); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func getReview(c *gin.Context) {
	id := c.Param("id")

	_, span := tracer.Start(c.Request.Context(), "query-database", oteltrace.WithAttributes(attribute.String("id", id)))
	review, err := database.Get[models.Review]("reviews", id)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if review == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, review); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
