package server

import (
	"net/http"
	"net/url"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/api-core.go/tracing"
	apiutil "github.com/sweetrpg/api-core.go/util"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var publisherPatchConfig = entityPatchConfig[vo.PublisherVO]{
	recordType: "publisher",
	get: func(c *gin.Context, id string) (*vo.PublisherVO, error) {
		return data.GetPublisher(c.Request.Context(), id)
	},
	update: func(c *gin.Context, id string, entity *vo.PublisherVO) (*vo.PublisherVO, error) {
		return data.UpdatePublisher(c.Request.Context(), id, entity)
	},
	setUpdatedBy: func(v *vo.PublisherVO, by string) { v.UpdatedBy = by },
	fields: map[string]entityFieldAccessor[vo.PublisherVO]{
		"name": {
			get: func(v *vo.PublisherVO) string { return v.Name },
			set: func(v *vo.PublisherVO, s string) { v.Name = s },
		},
		"address": {
			get: func(v *vo.PublisherVO) string { return v.Address },
			set: func(v *vo.PublisherVO, s string) { v.Address = s },
		},
		"website": {
			get: func(v *vo.PublisherVO) string { return v.Website.String() },
			set: func(v *vo.PublisherVO, s string) {
				if u, err := url.Parse(s); err == nil && u != nil {
					v.Website = *u
				}
			},
		},
		"notes": {
			get: func(v *vo.PublisherVO) string { return v.Notes },
			set: func(v *vo.PublisherVO, s string) { v.Notes = s },
		},
	},
}

func setupPublisherHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client) {
	logging.Logger.Info("Setting up publisher endpoint handlers...")
	ttl := ttls.TTL("publishers")
	g.GET("/publishers", cache.CachePage(store, ttl, listPublishers))
	g.GET("/publishers/:id", cache.CachePage(store, ttl, getPublisher))
	g.GET("/publishers/:id/volumes", cache.CachePage(store, ttl, getPublisherVolumes))

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.PATCH("/publishers/:id", writeRoles, patchEntity(publisherPatchConfig))

	reviewRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)
	g.GET("/publishers/:id/proposed-changes", reviewRoles, listEntityProposedChanges("publisher"))
	g.POST("/publishers/:id/proposed-changes/:proposalId/accept", reviewRoles, acceptEntityProposedChange(publisherPatchConfig))
	g.POST("/publishers/:id/proposed-changes/:proposalId/reject", reviewRoles, rejectEntityProposedChange("publisher"))
}

// List publishers.
//
//	@Summary		List publishers
//	@Description	Lists the publishers in the database.
//	@Tags			publishers
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/publishers [get]
func listPublishers(c *gin.Context) {
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "publishers", "list-publishers", params)
	vos, err := data.QueryPublishers(c.Request.Context(), params)
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

// Get publisher volumes.
//
//	@Summary		Get publisher volumes
//	@Description	Gets all the volumes associated with a particular publisher
//	@Tags			publishers
//	@Produce		json
//	@Param			id		path		string			true	"Publisher ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/publishers/{id}/volumes [get]
func getPublisherVolumes(c *gin.Context) {
	id := c.Param("id")

	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	inOp := "$in"
	params.Filter = []apiutil.Filter{{
		Field:     "publisher_ids",
		Operation: &inOp,
		Value:     []string{id},
	}}

	span := tracing.BuildSpanWithParams(c.Request.Context(), "publishers", "list-publisher-volumes", params)
	vos, err := data.QueryVolumes(c.Request.Context(), params)
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

// Get a publisher.
//
//	@Summary		Get a publisher
//	@Description	Get the details of a publisher from the database.
//	@Tags			publishers
//	@Produce		json
//	@Param			id		path		string			true	"Publisher ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/publishers/{id} [get]
func getPublisher(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("publishers").Start(c.Request.Context(), "get-publisher", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetPublisher(c.Request.Context(), id)
	span.End()
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
