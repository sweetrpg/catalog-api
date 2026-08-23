package server

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/api-core.go/tracing"
	apiutil "github.com/sweetrpg/api-core.go/util"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupReviewHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config) {
	logging.Logger.Info("Setting up review endpoint handlers...")
	ttl := ttls.TTL("reviews")
	g.GET("/reviews", cache.CachePage(store, ttl, listReviews))
	g.GET("/reviews/:id", cache.CachePage(store, ttl, getReview))
	// g.GET("/reviews/:id/reviews", cache.CachePage(store, ttl, getReviewReviews))
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
	logging.Logger.Debug("listReviews: enter", "params", params)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "reviews", "list-reviews", params)
	vos, err := data.QueryReviews(c.Request.Context(), params)
	span.End()
	if err != nil {
		logging.Logger.Error("listReviews: query failed", "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	logging.Logger.Debug("listReviews: exit", "count", len(vos))

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
	logging.Logger.Debug("getReview: enter", "id", id)

	_, span := otel.Tracer("reviews").Start(c.Request.Context(), "get-review", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetReview(c.Request.Context(), id)
	span.End()
	if err != nil {
		logging.Logger.Error("getReview: query failed", "id", id, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if vo == nil {
		logging.Logger.Debug("getReview: exit", "id", id, "outcome", "not_found")
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	logging.Logger.Debug("getReview: exit", "id", id, "outcome", "ok")

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, vo); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
