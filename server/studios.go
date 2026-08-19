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

var studioVersionConfig = entityVersionAPIConfig[vo.StudioVO, vo.StudioVersionVO]{
	recordType: "studio",
	get: func(c *gin.Context, id string) (*vo.StudioVO, error) {
		return data.GetStudio(c.Request.Context(), id)
	},
	create: func(c *gin.Context, entity *vo.StudioVO, createdBy string) (*string, error) {
		entity.CreatedBy = createdBy
		return data.AddStudio(c.Request.Context(), entity)
	},
	createVersion: func(c *gin.Context, id string, entity *vo.StudioVO, state catalogmodels.VersionState) (*vo.StudioVersionVO, error) {
		return data.UpdateStudio(c.Request.Context(), id, entity, state)
	},
	listVersions: func(c *gin.Context, id string) ([]*vo.StudioVersionVO, error) {
		return data.ListStudioVersions(c.Request.Context(), id)
	},
	getVersion: func(c *gin.Context, id string, version int) (*vo.StudioVersionVO, error) {
		return data.GetStudioVersion(c.Request.Context(), id, version)
	},
	acceptVersion: func(c *gin.Context, id string, version int, selectedFields []string, reviewedBy string, reviewNote *string) (*vo.StudioVersionVO, []string, error) {
		return data.AcceptStudioVersion(c.Request.Context(), id, version, selectedFields, reviewedBy, reviewNote)
	},
	rejectVersion: func(c *gin.Context, id string, version int, reviewedBy string, reviewNote *string) error {
		return data.RejectStudioVersion(c.Request.Context(), id, version, reviewedBy, reviewNote)
	},
	retractVersion: func(c *gin.Context, id string, version int, submitterID string) (*vo.StudioVersionVO, error) {
		return data.RetractStudioVersion(c.Request.Context(), id, version, submitterID)
	},
	setCurrentVersion: func(c *gin.Context, id string, version int) (*vo.StudioVersionVO, error) {
		return data.SetCurrentStudioVersion(c.Request.Context(), id, version)
	},
	versionState:  func(v *vo.StudioVersionVO) string { return string(v.State) },
	versionNumber: func(v *vo.StudioVersionVO) int { return v.Version },
	fields: map[string]entityFieldAccessor[vo.StudioVO]{
		"name": {
			get: func(v *vo.StudioVO) string { return v.Name },
			set: func(v *vo.StudioVO, s string) { v.Name = s },
		},
		// Website is a plain string on the VO (catalog-objects.go v0.4.1+) - no url.Parse
		// round-trip needed here anymore.
		"website": {
			get: func(v *vo.StudioVO) string { return v.Website },
			set: func(v *vo.StudioVO, s string) { v.Website = s },
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
	g.GET("/studios/:id/versions", listEntityVersions(studioVersionConfig))
	g.GET("/studios/:id/versions/:version", getEntityVersion(studioVersionConfig))

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.POST("/studios", writeRoles, createEntityVersion(studioVersionConfig))
	g.PATCH("/studios/:id", writeRoles, patchEntityVersion(studioVersionConfig))
	g.POST("/studios/:id/versions/:version/retract", writeRoles, retractEntityVersion(studioVersionConfig))

	reviewRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)
	g.POST("/studios/:id/versions/:version/accept", reviewRoles, acceptEntityVersion(studioVersionConfig))
	g.POST("/studios/:id/versions/:version/reject", reviewRoles, rejectEntityVersion(studioVersionConfig))

	rollbackRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin)
	g.POST("/studios/:id/versions/:version/current", rollbackRoles, setCurrentEntityVersion(studioVersionConfig))
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
