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

var systemVersionConfig = entityVersionAPIConfig[vo.SystemVO, vo.SystemVersionVO]{
	recordType: "system",
	listPath:   "/systems",
	get: func(c *gin.Context, id string) (*vo.SystemVO, error) {
		return data.GetSystem(c.Request.Context(), id)
	},
	createVersion: func(c *gin.Context, id string, entity *vo.SystemVO, state catalogmodels.VersionState) (*vo.SystemVersionVO, error) {
		return data.UpdateSystem(c.Request.Context(), id, entity, state)
	},
	listVersions: func(c *gin.Context, id string) ([]*vo.SystemVersionVO, error) {
		return data.ListSystemVersions(c.Request.Context(), id)
	},
	getVersion: func(c *gin.Context, id string, version int) (*vo.SystemVersionVO, error) {
		return data.GetSystemVersion(c.Request.Context(), id, version)
	},
	acceptVersion: func(c *gin.Context, id string, version int, selectedFields []string, reviewedBy string, reviewNote *string) (*vo.SystemVersionVO, []string, error) {
		return data.AcceptSystemVersion(c.Request.Context(), id, version, selectedFields, reviewedBy, reviewNote)
	},
	rejectVersion: func(c *gin.Context, id string, version int, reviewedBy string, reviewNote *string) error {
		return data.RejectSystemVersion(c.Request.Context(), id, version, reviewedBy, reviewNote)
	},
	retractVersion: func(c *gin.Context, id string, version int, submitterID string) (*vo.SystemVersionVO, error) {
		return data.RetractSystemVersion(c.Request.Context(), id, version, submitterID)
	},
	setCurrentVersion: func(c *gin.Context, id string, version int) (*vo.SystemVersionVO, error) {
		return data.SetCurrentSystemVersion(c.Request.Context(), id, version)
	},
	softDelete: func(c *gin.Context, id string, deletedBy string) error {
		return data.SoftDeleteSystem(c.Request.Context(), id, deletedBy)
	},
	restore: func(c *gin.Context, id string) error {
		return data.RestoreSystem(c.Request.Context(), id)
	},
	versionState:  func(v *vo.SystemVersionVO) string { return string(v.State) },
	versionNumber: func(v *vo.SystemVersionVO) int { return v.Version },
	fields: map[string]entityFieldAccessor[vo.SystemVO]{
		"game_system": {
			get: func(v *vo.SystemVO) string { return v.GameSystem },
			set: func(v *vo.SystemVO, s string) { v.GameSystem = s },
		},
		"edition": {
			get: func(v *vo.SystemVO) string { return v.Edition },
			set: func(v *vo.SystemVO, s string) { v.Edition = s },
		},
		"notes": {
			get: func(v *vo.SystemVO) string { return v.Notes },
			set: func(v *vo.SystemVO, s string) { v.Notes = s },
		},
	},
}

func setupSystemHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client) {
	logging.Logger.Info("Setting up system endpoint handlers...")
	ttl := ttls.TTL("systems")
	g.GET("/systems", cache.CachePage(store, ttl, listSystems))
	g.GET("/systems/:id", cache.CachePage(store, ttl, getSystem))
	g.GET("/systems/:id/versions", listEntityVersions(systemVersionConfig))
	g.GET("/systems/:id/versions/:version", getEntityVersion(systemVersionConfig))

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.PATCH("/systems/:id", writeRoles, patchEntityVersion(systemVersionConfig, store))
	g.POST("/systems/:id/versions/:version/retract", writeRoles, retractEntityVersion(systemVersionConfig))

	reviewRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)
	g.POST("/systems/:id/versions/:version/accept", reviewRoles, acceptEntityVersion(systemVersionConfig, store))
	g.POST("/systems/:id/versions/:version/reject", reviewRoles, rejectEntityVersion(systemVersionConfig))

	rollbackRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin)
	g.POST("/systems/:id/versions/:version/current", rollbackRoles, setCurrentEntityVersion(systemVersionConfig))
	g.DELETE("/systems/:id", rollbackRoles, deleteEntity(systemVersionConfig, store))
	g.POST("/systems/:id/restore", rollbackRoles, restoreEntity(systemVersionConfig, store))
}

// List systems.
//
//	@Summary		List systems
//	@Description	Lists the systems in the database.
//	@Tags			systems
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/systems [get]
func listSystems(c *gin.Context) {
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "systems", "list-systems", params)
	vos, err := data.QuerySystems(c.Request.Context(), params)
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

// Get a system.
//
//	@Summary		Get a system
//	@Description	Get the details of a system from the database.
//	@Tags			systems
//	@Produce		json
//	@Param			id		path		string			true	"System ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/systems/{id} [get]
func getSystem(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("systems").Start(c.Request.Context(), "get-system", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetSystem(c.Request.Context(), id)
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
