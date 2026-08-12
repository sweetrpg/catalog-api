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

var studioPatchConfig = entityPatchConfig[vo.StudioVO]{
	recordType: "studio",
	get: func(c *gin.Context, id string) (*vo.StudioVO, error) {
		return data.GetStudio(c.Request.Context(), id)
	},
	update: func(c *gin.Context, id string, entity *vo.StudioVO) (*vo.StudioVO, error) {
		return data.UpdateStudio(c.Request.Context(), id, entity)
	},
	setUpdatedBy: func(v *vo.StudioVO, by string) { v.UpdatedBy = by },
	fields: map[string]entityFieldAccessor[vo.StudioVO]{
		"name": {
			get: func(v *vo.StudioVO) string { return v.Name },
			set: func(v *vo.StudioVO, s string) { v.Name = s },
		},
		"website": {
			get: func(v *vo.StudioVO) string { return v.Website.String() },
			set: func(v *vo.StudioVO, s string) {
				if u, err := url.Parse(s); err == nil && u != nil {
					v.Website = *u
				}
			},
		},
		"notes": {
			get: func(v *vo.StudioVO) string { return v.Notes },
			set: func(v *vo.StudioVO, s string) { v.Notes = s },
		},
	},
}

func setupStudioHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client) {
	logging.Logger.Info("Setting up studio endpoint handlers...")
	ttl := ttls.TTL("studios")
	g.GET("/studios", cache.CachePage(store, ttl, listStudios))
	g.GET("/studios/:id", cache.CachePage(store, ttl, getStudio))
	g.GET("/studios/:id/volumes", cache.CachePage(store, ttl, getStudioVolumes))

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.PATCH("/studios/:id", writeRoles, patchEntity(studioPatchConfig))

	reviewRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)
	g.GET("/studios/:id/proposed-changes", reviewRoles, listEntityProposedChanges("studio"))
	g.POST("/studios/:id/proposed-changes/:proposalId/accept", reviewRoles, acceptEntityProposedChange(studioPatchConfig))
	g.POST("/studios/:id/proposed-changes/:proposalId/reject", reviewRoles, rejectEntityProposedChange("studio"))
}

// List studios.
//
//	@Summary		List studios
//	@Description	Lists the studios in the database.
//	@Tags			studios
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/studios [get]
func listStudios(c *gin.Context) {
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "studios", "list-studios", params)
	vos, err := data.QueryStudios(c.Request.Context(), params)
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

// Get studio volumes.
//
//	@Summary		Get studio volumes
//	@Description	Gets all the volumes associated with a particular studio
//	@Tags			studios
//	@Produce		json
//	@Param			id		path		string			true	"Studio ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/studios/{id}/volumes [get]
func getStudioVolumes(c *gin.Context) {
	id := c.Param("id")

	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	inOp := "$in"
	params.Filter = []apiutil.Filter{{
		Field:     "studio_ids",
		Operation: &inOp,
		Value:     []string{id},
	}}

	span := tracing.BuildSpanWithParams(c.Request.Context(), "studios", "list-studio-volumes", params)
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

// Get a studio.
//
//	@Summary		Get a studio
//	@Description	Get the details of a studio from the database.
//	@Tags			studios
//	@Produce		json
//	@Param			id		path		string			true	"Studio ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/studios/{id} [get]
func getStudio(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("studios").Start(c.Request.Context(), "get-studio", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetStudio(c.Request.Context(), id)
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
