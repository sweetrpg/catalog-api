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
	options "go.jtlabs.io/query"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupLicenseHandlers(g *gin.Engine, store persistence.CacheStore) {
	logging.Logger.Info("Setting up license endpoint handlers...")
	g.GET("/licenses", cache.CachePage(store, time.Hour, listLicenses))
	g.GET("/licenses/:id", cache.CachePage(store, time.Hour, getLicense))
	g.GET("/licenses/:id/volumes", cache.CachePage(store, time.Hour, getLicenseVolumes))
}

// List licenses.
//
//	@Summary		List licenses
//	@Description	Lists the licenses in the database.
//	@Tags			licenses
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/licenses [get]
func listLicenses(c *gin.Context) {
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "licenses", "list-licenses", params)
	vos, err := data.GetLicenses(c.Request.Context(), params)
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

// Get license volumes.
//
//	@Summary		Get license volumes
//	@Description	Gets all the volumes associated with a particular license
//	@Tags			licenses
//	@Produce		json
//	@Param			id		path		string			true	"License ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/licenses/{id}/volumes [get]
func getLicenseVolumes(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	opt, _ := options.FromQuerystring(c.Request.URL.RawQuery)

	filter := bson.D{
		{"license_ids",
			bson.D{{"$in", id}},
		},
	}

	span := tracing.BuildSpanWithOptions(c.Request.Context(), "licenses", "list-license-volumes", opt)
	vos, err := data.GetVolumes(c.Request.Context(), filter, opt)
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

// Get a license.
//
//	@Summary		Get a license
//	@Description	Get the details of a license from the database.
//	@Tags			licenses
//	@Produce		json
//	@Param			id		path		string			true	"License ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/licenses/{id} [get]
func getLicense(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("licenses").Start(c.Request.Context(), "get-license", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetVolume(c.Request.Context(), id)
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
