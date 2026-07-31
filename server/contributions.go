package server

import (
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/api-core.go/tracing"
	apiutil "github.com/sweetrpg/api-core.go/util"
	_ "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupContributionHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config) {
	logging.Logger.Info("Setting up contribution endpoint handlers...")
	ttl := ttls.TTL("contributions")
	g.GET("/contributions", cache.CachePage(store, ttl, listContributions))
	g.GET("/contributions/:id", cache.CachePage(store, ttl, getContribution))
	// g.GET("/contributions/:id/contributions", cache.CachePage(store, ttl, getContributionContributions))
}

// List contributions.
//
//	@Summary		List contributions
//	@Description	Lists the contributions in the database.
//	@Tags			contributions
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	vo.ErrorVO
//	@Router			/contributions [get]
func listContributions(c *gin.Context) {
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "contributions", "list-contributions", params)
	vos, err := data.QueryContributions(c.Request.Context(), params)
	span.End()
	logging.Logger.Info(fmt.Sprintf("vos=%v", vos))
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

// Get a contribution.
//
//	@Summary		Get a contribution
//	@Description	Get the details of a contribution from the database.
//	@Tags			contributions
//	@Produce		json
//	@Param			id		path		string			true	"Contribution ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/contributions/{id} [get]
func getContribution(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("contributions").Start(c.Request.Context(), "get-contribution", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetContribution(c.Request.Context(), id)
	span.End()
	logging.Logger.Info(fmt.Sprintf("vo=%v", vo))
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if vo == nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, vo); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
