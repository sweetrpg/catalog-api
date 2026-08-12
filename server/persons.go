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
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var personPatchConfig = entityPatchConfig[vo.PersonVO]{
	recordType: "person",
	get: func(c *gin.Context, id string) (*vo.PersonVO, error) {
		return data.GetPerson(c.Request.Context(), id)
	},
	update: func(c *gin.Context, id string, entity *vo.PersonVO) (*vo.PersonVO, error) {
		return data.UpdatePerson(c.Request.Context(), id, entity)
	},
	setUpdatedBy: func(v *vo.PersonVO, by string) { v.UpdatedBy = by },
	fields: map[string]entityFieldAccessor[vo.PersonVO]{
		"name": {
			get: func(v *vo.PersonVO) string { return v.Name },
			set: func(v *vo.PersonVO, s string) { v.Name = s },
		},
		"notes": {
			get: func(v *vo.PersonVO) string { return v.Notes },
			set: func(v *vo.PersonVO, s string) { v.Notes = s },
		},
	},
}

func setupPersonHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client) {
	logging.Logger.Info("Setting up person endpoint handlers...")
	ttl := ttls.TTL("persons")
	g.GET("/persons", cache.CachePage(store, ttl, listPersons))
	g.GET("/persons/:id", cache.CachePage(store, ttl, getPerson))
	g.GET("/persons/:id/volumes", cache.CachePage(store, ttl, getPersonVolumes))

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.PATCH("/persons/:id", writeRoles, patchEntity(personPatchConfig))

	reviewRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)
	g.GET("/persons/:id/proposed-changes", reviewRoles, listEntityProposedChanges("person"))
	g.POST("/persons/:id/proposed-changes/:proposalId/accept", reviewRoles, acceptEntityProposedChange(personPatchConfig))
	g.POST("/persons/:id/proposed-changes/:proposalId/reject", reviewRoles, rejectEntityProposedChange("person"))
}

// List persons.
//
//	@Summary		List persons
//	@Description	Lists the persons in the database.
//	@Tags			persons
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/persons [get]
func listPersons(c *gin.Context) {
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "persons", "list-persons", params)
	vos, err := data.QueryPersons(c.Request.Context(), params)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, vos); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// Get person volumes.
//
//	@Summary		Get person volumes
//	@Description	Gets all the volumes a particular person has contributed to
//	@Tags			persons
//	@Produce		json
//	@Param			id		path		string			true	"Person ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/persons/{id}/volumes [get]
func getPersonVolumes(c *gin.Context) {
	id := c.Param("id")

	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)
	inOp := "$in"
	params.Filter = []apiutil.Filter{{
		Field:     "person_id",
		Operation: &inOp,
		Value:     []string{id},
	}}

	span := tracing.BuildSpanWithParams(c.Request.Context(), "persons", "list-person-volumes", params)
	contributions, err := data.QueryContributions(c.Request.Context(), params)
	span.End()
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	seen := make(map[string]bool, len(contributions))
	vos := make([]*vo.VolumeVO, 0, len(contributions))
	for _, contribution := range contributions {
		if contribution.Volume == nil || seen[contribution.Volume.ID] {
			continue
		}
		seen[contribution.Volume.ID] = true
		vos = append(vos, contribution.Volume)
	}

	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, vos); err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// Get a person.
//
//	@Summary		Get a person
//	@Description	Get the details of a person from the database.
//	@Tags			persons
//	@Produce		json
//	@Param			id		path		string			true	"Person ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/persons/{id} [get]
func getPerson(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("persons").Start(c.Request.Context(), "get-person", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetPerson(c.Request.Context(), id)
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
