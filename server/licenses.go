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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var licensePatchConfig = entityPatchConfig[vo.LicenseVO]{
	recordType: "license",
	get: func(c *gin.Context, id string) (*vo.LicenseVO, error) {
		return data.GetLicense(c.Request.Context(), id)
	},
	update: func(c *gin.Context, id string, entity *vo.LicenseVO) (*vo.LicenseVO, error) {
		return data.UpdateLicense(c.Request.Context(), id, entity)
	},
	setUpdatedBy: func(v *vo.LicenseVO, by string) { v.UpdatedBy = by },
	fields: map[string]entityFieldAccessor[vo.LicenseVO]{
		"title": {
			get: func(v *vo.LicenseVO) string { return v.Title },
			set: func(v *vo.LicenseVO, s string) { v.Title = s },
		},
		"short_title": {
			get: func(v *vo.LicenseVO) string { return v.ShortTitle },
			set: func(v *vo.LicenseVO, s string) { v.ShortTitle = s },
		},
		"version": {
			get: func(v *vo.LicenseVO) string { return v.Version },
			set: func(v *vo.LicenseVO, s string) { v.Version = s },
		},
		"deed": {
			get: func(v *vo.LicenseVO) string { return v.Deed },
			set: func(v *vo.LicenseVO, s string) { v.Deed = s },
		},
		"legal_code": {
			get: func(v *vo.LicenseVO) string { return v.LegalCode },
			set: func(v *vo.LicenseVO, s string) { v.LegalCode = s },
		},
		"website": {
			get: func(v *vo.LicenseVO) string { return v.Website.String() },
			set: func(v *vo.LicenseVO, s string) {
				if u, err := url.Parse(s); err == nil && u != nil {
					v.Website = *u
				}
			},
		},
		"status": {
			get: func(v *vo.LicenseVO) string { return v.Status },
			set: func(v *vo.LicenseVO, s string) { v.Status = s },
		},
		"availability": {
			get: func(v *vo.LicenseVO) string { return v.Availability },
			set: func(v *vo.LicenseVO, s string) { v.Availability = s },
		},
		"notes": {
			get: func(v *vo.LicenseVO) string { return v.Notes },
			set: func(v *vo.LicenseVO, s string) { v.Notes = s },
		},
	},
}

func setupLicenseHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client) {
	logging.Logger.Info("Setting up license endpoint handlers...")
	ttl := ttls.TTL("licenses")
	g.GET("/licenses", cache.CachePage(store, ttl, listLicenses))
	g.GET("/licenses/:id", cache.CachePage(store, ttl, getLicense))
	g.GET("/licenses/:id/volumes", cache.CachePage(store, ttl, getLicenseVolumes))

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.PATCH("/licenses/:id", writeRoles, patchEntity(licensePatchConfig))

	reviewRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)
	g.GET("/licenses/:id/proposed-changes", reviewRoles, listEntityProposedChanges("license"))
	g.POST("/licenses/:id/proposed-changes/:proposalId/accept", reviewRoles, acceptEntityProposedChange(licensePatchConfig))
	g.POST("/licenses/:id/proposed-changes/:proposalId/reject", reviewRoles, rejectEntityProposedChange("license"))
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
	vos, err := data.QueryLicenses(c.Request.Context(), params)
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

	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	inOp := "$in"
	params.Filter = []apiutil.Filter{{
		Field:     "license_ids",
		Operation: &inOp,
		Value:     []string{id.Hex()},
	}}

	span := tracing.BuildSpanWithParams(c.Request.Context(), "licenses", "list-license-volumes", params)
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
	vo, err := data.GetLicense(c.Request.Context(), id)
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
