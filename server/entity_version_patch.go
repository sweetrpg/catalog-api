package server

import (
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
)

// entityFieldAccessor reads/writes one patchable string field on a live entity VO - used to
// build the "updated" record from a PATCH request's present, known fields.
type entityFieldAccessor[T any] struct {
	get func(*T) string
	set func(*T, string)
}

// entityVersionAPIConfig wires the generic version-based patch/review/rollback flow below (shared
// across publisher, studio, person, license, and system - the meta+version model
// volumes_patch.go/volumes_versions.go implement bespoke for volumes, predating this shared
// type) to one entity type's real data-layer calls and patchable field accessors. This replaced
// the former entity_patch.go's proposed_changes-based generic mechanism once every type using it
// migrated onto the version model (task group 7).
type entityVersionAPIConfig[T any, V any] struct {
	recordType        string
	fields            map[string]entityFieldAccessor[T]
	get               func(c *gin.Context, id string) (*T, error)
	createVersion     func(c *gin.Context, id string, entity *T, state catalogmodels.VersionState) (*V, error)
	listVersions      func(c *gin.Context, id string) ([]*V, error)
	getVersion        func(c *gin.Context, id string, version int) (*V, error)
	acceptVersion     func(c *gin.Context, id string, version int, selectedFields []string, reviewedBy string, reviewNote *string) (*V, []string, error)
	rejectVersion     func(c *gin.Context, id string, version int, reviewedBy string, reviewNote *string) error
	retractVersion    func(c *gin.Context, id string, version int, submitterID string) (*V, error)
	setCurrentVersion func(c *gin.Context, id string, version int) (*V, error)
	versionState      func(*V) string
	versionNumber     func(*V) int
}

// patchEntityVersion edits an entity, or submits an edit for review, creating a version either
// way - the version-model counterpart of patchEntity.
func patchEntityVersion[T any, V any](cfg entityVersionAPIConfig[T, V]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req map[string]*string
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
			return
		}
		hasKnownField := false
		for field, value := range req {
			if _, known := cfg.fields[field]; known && value != nil {
				hasKnownField = true
				break
			}
		}
		if !hasKnownField {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "no recognized fields to change"})
			return
		}

		existing, err := cfg.get(c, id)
		if err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
			return
		}
		if existing == nil {
			c.JSON(http.StatusNotFound, apiv.ErrorVO{})
			return
		}

		updated := *existing
		for field, value := range req {
			accessor, known := cfg.fields[field]
			if !known || value == nil {
				continue
			}
			accessor.set(&updated, *value)
		}

		roles := authz.Roles(c)
		state := catalogmodels.VersionStateSubmitted
		if authz.HasRole(roles, authz.RoleAdmin) || authz.HasRole(roles, authz.RoleEditor) {
			state = catalogmodels.VersionStateLive
		}

		version, err := cfg.createVersion(c, id, &updated, state)
		if err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "update_failed", Message: err.Error()})
			return
		}
		if version == nil {
			c.JSON(http.StatusNotFound, apiv.ErrorVO{})
			return
		}

		if state != catalogmodels.VersionStateLive {
			c.JSON(http.StatusAccepted, submittedVersionResponse{
				Version: cfg.versionNumber(version), State: cfg.versionState(version),
				Message: "Change submitted for review",
			})
			return
		}

		result, err := cfg.get(c, id)
		if err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
			return
		}
		c.Writer.Header().Set("Content-type", jsonapi.MediaType)
		c.Writer.WriteHeader(http.StatusOK)
		if err := jsonapi.MarshalPayload(c.Writer, result); err != nil {
			sentry.CaptureException(err)
		}
	}
}

func entityVersionParam(c *gin.Context) (int, bool) {
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version < 1 {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "version must be a positive integer"})
		return 0, false
	}
	return version, true
}

// listEntityVersions lists a record's version history, newest first.
func listEntityVersions[T any, V any](cfg entityVersionAPIConfig[T, V]) gin.HandlerFunc {
	return func(c *gin.Context) {
		versions, err := cfg.listVersions(c, c.Param("id"))
		if err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, versions)
	}
}

// getEntityVersion fetches one version's full field snapshot.
func getEntityVersion[T any, V any](cfg entityVersionAPIConfig[T, V]) gin.HandlerFunc {
	return func(c *gin.Context) {
		version, ok := entityVersionParam(c)
		if !ok {
			return
		}
		result, err := cfg.getVersion(c, c.Param("id"), version)
		if err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
			return
		}
		if result == nil {
			c.JSON(http.StatusNotFound, apiv.ErrorVO{})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// acceptEntityVersion accepts a submitted version, in full or in part.
func acceptEntityVersion[T any, V any](cfg entityVersionAPIConfig[T, V]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		version, ok := entityVersionParam(c)
		if !ok {
			return
		}
		var req acceptVersionRequest
		if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
			return
		}
		var selectedFields []string
		if req.Fields != nil {
			selectedFields = *req.Fields
		}
		accepted, conflicts, err := cfg.acceptVersion(c, id, version, selectedFields, authz.Subject(c), nil)
		if err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "accept_failed", Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, reviewVersionResponse{
			Version: cfg.versionNumber(accepted), State: cfg.versionState(accepted), Conflicts: conflicts,
		})
	}
}

// rejectEntityVersion rejects a submitted version in full.
func rejectEntityVersion[T any, V any](cfg entityVersionAPIConfig[T, V]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		version, ok := entityVersionParam(c)
		if !ok {
			return
		}
		var req rejectVersionRequest
		if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
			return
		}
		var note *string
		if req.Note != "" {
			note = &req.Note
		}
		if err := cfg.rejectVersion(c, id, version, authz.Subject(c), note); err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "reject_failed", Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, reviewVersionResponse{Version: version, State: "rejected"})
	}
}

// retractEntityVersion lets the original submitter withdraw their own pending submission.
func retractEntityVersion[T any, V any](cfg entityVersionAPIConfig[T, V]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		version, ok := entityVersionParam(c)
		if !ok {
			return
		}
		retracted, err := cfg.retractVersion(c, id, version, authz.Subject(c))
		if err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "retract_failed", Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, reviewVersionResponse{Version: cfg.versionNumber(retracted), State: cfg.versionState(retracted)})
	}
}

// setCurrentEntityVersion rolls a record back (or forward) to an arbitrary existing version.
func setCurrentEntityVersion[T any, V any](cfg entityVersionAPIConfig[T, V]) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		version, ok := entityVersionParam(c)
		if !ok {
			return
		}
		result, err := cfg.setCurrentVersion(c, id, version)
		if err != nil {
			sentry.CaptureException(err)
			c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "rollback_failed", Message: err.Error()})
			return
		}
		if result == nil {
			c.JSON(http.StatusNotFound, apiv.ErrorVO{})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
