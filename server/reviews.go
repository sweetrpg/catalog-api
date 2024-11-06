package server

import (
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/api-core/tracing"
	apiutil "github.com/sweetrpg/api-core/util"
	"github.com/sweetrpg/catalog-data/data"
	"github.com/sweetrpg/common/logging"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupReviewHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up review endpoint handlers...")
	g.GET("/reviews", cache.CachePage(store, time.Hour, listReviews))
	g.GET("/reviews/:id", cache.CachePage(store, time.Hour, getReview))
	// g.GET("/reviews/:id/reviews", cache.CachePage(store, time.Hour, getReviewReviews))
}

// List reviews.
//
//	@Summary		List reviews
//	@Description	Lists the reviews in the database.
//	@Tags			reviews
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/reviews [get]
func listReviews(c *gin.Context) {
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "reviews", "list-reviews", params)
	vos, err := data.GetReviews(c.Request.Context(), bson.D{}, params)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, vos); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// Get a review.
//
//	@Summary		Get a review
//	@Description	Get the details of a review from the database.
//	@Tags			reviews
//	@Produce		json
//	@Param			id		path		string			true	"Review ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/reviews/{id} [get]
func getReview(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("reviews").Start(c.Request.Context(), "get-review", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetReview(c.Request.Context(), id)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if vo == nil {
		c.JSON(http.StatusNotFound, gin.H{})
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, vo); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
