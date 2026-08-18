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
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var publisherVersionConfig = entityVersionAPIConfig[vo.PublisherVO, vo.PublisherVersionVO]{
	recordType: "publisher",
	get: func(c *gin.Context, id string) (*vo.PublisherVO, error) {
		return data.GetPublisher(c.Request.Context(), id)
	},
	createVersion: func(c *gin.Context, id string, entity *vo.PublisherVO, state catalogmodels.VersionState) (*vo.PublisherVersionVO, error) {
		return data.UpdatePublisher(c.Request.Context(), id, entity, state)
	},
	listVersions: func(c *gin.Context, id string) ([]*vo.PublisherVersionVO, error) {
		return data.ListPublisherVersions(c.Request.Context(), id)
	},
	getVersion: func(c *gin.Context, id string, version int) (*vo.PublisherVersionVO, error) {
		return data.GetPublisherVersion(c.Request.Context(), id, version)
	},
	acceptVersion: func(c *gin.Context, id string, version int, selectedFields []string, reviewedBy string, reviewNote *string) (*vo.PublisherVersionVO, []string, error) {
		return data.AcceptPublisherVersion(c.Request.Context(), id, version, selectedFields, reviewedBy, reviewNote)
	},
	rejectVersion: func(c *gin.Context, id string, version int, reviewedBy string, reviewNote *string) error {
		return data.RejectPublisherVersion(c.Request.Context(), id, version, reviewedBy, reviewNote)
	},
	retractVersion: func(c *gin.Context, id string, version int, submitterID string) (*vo.PublisherVersionVO, error) {
		return data.RetractPublisherVersion(c.Request.Context(), id, version, submitterID)
	},
	setCurrentVersion: func(c *gin.Context, id string, version int) (*vo.PublisherVersionVO, error) {
		return data.SetCurrentPublisherVersion(c.Request.Context(), id, version)
	},
	versionState:  func(v *vo.PublisherVersionVO) string { return string(v.State) },
	versionNumber: func(v *vo.PublisherVersionVO) int { return v.Version },
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
			get: func(v *vo.PublisherVO) string { return v.Website },
			set: func(v *vo.PublisherVO, s string) { v.Website = s },
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
	g.GET("/publishers/:id/versions", listEntityVersions(publisherVersionConfig))
	g.GET("/publishers/:id/versions/:version", getEntityVersion(publisherVersionConfig))

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.PATCH("/publishers/:id", writeRoles, patchEntityVersion(publisherVersionConfig))
	g.POST("/publishers/:id/versions/:version/retract", writeRoles, retractEntityVersion(publisherVersionConfig))

	reviewRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)
	g.POST("/publishers/:id/versions/:version/accept", reviewRoles, acceptEntityVersion(publisherVersionConfig))
	g.POST("/publishers/:id/versions/:version/reject", reviewRoles, rejectEntityVersion(publisherVersionConfig))

	rollbackRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin)
	g.POST("/publishers/:id/versions/:version/current", rollbackRoles, setCurrentEntityVersion(publisherVersionConfig))
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
