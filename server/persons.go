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
	"github.com/sweetrpg/catalog-api/internal/events"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var personVersionConfig = entityVersionAPIConfig[vo.PersonVO, vo.PersonVersionVO]{
	recordType: "person",
	listPath:   "/persons",
	get: func(c *gin.Context, id string) (*vo.PersonVO, error) {
		return data.GetPerson(c.Request.Context(), id)
	},
	create: func(c *gin.Context, entity *vo.PersonVO, createdBy string) (*string, error) {
		entity.CreatedBy = createdBy
		return data.AddPerson(c.Request.Context(), entity)
	},
	createVersion: func(c *gin.Context, id string, entity *vo.PersonVO, state catalogmodels.VersionState) (*vo.PersonVersionVO, error) {
		return data.UpdatePerson(c.Request.Context(), id, entity, state)
	},
	listVersions: func(c *gin.Context, id string) ([]*vo.PersonVersionVO, error) {
		return data.ListPersonVersions(c.Request.Context(), id)
	},
	getVersion: func(c *gin.Context, id string, version int) (*vo.PersonVersionVO, error) {
		return data.GetPersonVersion(c.Request.Context(), id, version)
	},
	acceptVersion: func(c *gin.Context, id string, version int, selectedFields []string, reviewedBy string, reviewNote *string) (*vo.PersonVersionVO, []string, error) {
		return data.AcceptPersonVersion(c.Request.Context(), id, version, selectedFields, reviewedBy, reviewNote)
	},
	rejectVersion: func(c *gin.Context, id string, version int, reviewedBy string, reviewNote *string) error {
		return data.RejectPersonVersion(c.Request.Context(), id, version, reviewedBy, reviewNote)
	},
	retractVersion: func(c *gin.Context, id string, version int, submitterID string) (*vo.PersonVersionVO, error) {
		return data.RetractPersonVersion(c.Request.Context(), id, version, submitterID)
	},
	setCurrentVersion: func(c *gin.Context, id string, version int) (*vo.PersonVersionVO, error) {
		return data.SetCurrentPersonVersion(c.Request.Context(), id, version)
	},
	softDelete: func(c *gin.Context, id string, deletedBy string) error {
		return data.SoftDeletePerson(c.Request.Context(), id, deletedBy)
	},
	restore: func(c *gin.Context, id string) error {
		return data.RestorePerson(c.Request.Context(), id)
	},
	versionState:  func(v *vo.PersonVersionVO) string { return string(v.State) },
	versionNumber: func(v *vo.PersonVersionVO) int { return v.Version },
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

func setupPersonHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client, eventPublisher *events.Publisher) {
	logging.Logger.Info("Setting up person endpoint handlers...")
	personVersionConfig.onPublishEvent = events.PublishPersonEvent(eventPublisher)
	ttl := ttls.TTL("persons")
	g.GET("/persons", cache.CachePage(store, ttl, listPersons))
	g.GET("/persons/search", cache.CachePage(store, ttl, searchPersons))
	g.GET("/persons/:id", cache.CachePage(store, ttl, getPerson))
	g.GET("/persons/:id/volumes", cache.CachePage(store, ttl, getPersonVolumes))
	g.GET("/persons/:id/versions", listEntityVersions(personVersionConfig))
	g.GET("/persons/:id/versions/:version", getEntityVersion(personVersionConfig))

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.POST("/persons", writeRoles, createEntityVersion(personVersionConfig, store))
	g.PATCH("/persons/:id", writeRoles, patchEntityVersion(personVersionConfig, store))
	g.POST("/persons/:id/versions/:version/retract", writeRoles, retractEntityVersion(personVersionConfig))

	reviewRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)
	g.POST("/persons/:id/versions/:version/accept", reviewRoles, acceptEntityVersion(personVersionConfig, store))
	g.POST("/persons/:id/versions/:version/reject", reviewRoles, rejectEntityVersion(personVersionConfig))
	// Bulk-create is narrower than single-entity create (writeRoles, above) - editor/admin only,
	// no submitter path, per catalog-entity-bulk-add's spec (matches reviewRoles' tier).
	g.POST("/persons/bulk", reviewRoles, bulkCreateEntityVersion(personVersionConfig))

	rollbackRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin)
	g.POST("/persons/:id/versions/:version/current", rollbackRoles, setCurrentEntityVersion(personVersionConfig))
	g.DELETE("/persons/:id", rollbackRoles, deleteEntity(personVersionConfig, store))
	g.POST("/persons/:id/restore", rollbackRoles, restoreEntity(personVersionConfig, store))
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

// Search persons.
//
//	@Summary		Search persons
//	@Description	Finds persons whose name contains the query string - backs autocomplete/picker inputs.
//	@Tags			persons
//	@Produce		json
//	@Param			q		query		string			true	"Search query"
//	@Success		200		{object}	interface{}
//	@Failure		400		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/persons/search [get]
func searchPersons(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}

	_, span := otel.Tracer("persons").Start(c.Request.Context(), "search-persons", oteltrace.WithAttributes(attribute.String("q", q)))
	vos, err := data.SearchPersons(c.Request.Context(), q)
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
