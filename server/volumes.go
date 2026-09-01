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
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-api/internal/events"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// volumeEventPublisher is the NATS JetStream publisher for volume events, set by setupVolumeHandlers.
var volumeEventPublisher *events.Publisher

func setupVolumeHandlers(g *gin.Engine, store persistence.CacheStore, ttls cachettl.Config, authzClient *authz.Client, assetsClient *assets.Client, editSessions *editsession.Store, eventPublisher *events.Publisher) {
	volumeEventPublisher = eventPublisher
	logging.Logger.Info("Setting up volume endpoint handlers...")
	ttl := ttls.TTL("volumes")
	g.GET("/volumes", cache.CachePage(store, ttl, listVolumes))
	g.GET("/volumes/search", cache.CachePage(store, ttl, searchVolumes))
	g.GET("/volumes/:id", cache.CachePage(store, ttl, getVolume))
	g.GET("/volumes/:id/versions", listVolumeVersions)
	g.GET("/volumes/:id/versions/:version", getVolumeVersion)
	// g.GET("/volumes/:id/volumes", cache.CachePage(store, ttl, getVolumeVolumes))

	writeRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor, authz.RoleSubmitter)
	g.PATCH("/volumes/:id", writeRoles, func(c *gin.Context) {
		patchVolume(c, store)
	})
	g.POST("/volumes/:id/finalize-session", writeRoles, func(c *gin.Context) {
		finalizeVolumeSession(c, assetsClient, editSessions, store)
	})

	reviewRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin, authz.RoleEditor)

	g.POST("/volumes/:id/versions/:version/accept", reviewRoles, func(c *gin.Context) {
		acceptVolumeVersion(c, assetsClient)
	})
	g.POST("/volumes/:id/versions/:version/reject", reviewRoles, rejectVolumeVersion)
	g.POST("/volumes/:id/versions/:version/retract", writeRoles, retractVolumeVersion)
	g.POST("/volumes/:id/versions/:version/pull-back", writeRoles, func(c *gin.Context) {
		pullBackVolumeVersion(c, editSessions)
	})

	rollbackRoles := authz.RequireAnyRole(authzClient, constants.ServiceName, authz.RoleAdmin)
	g.POST("/volumes/:id/versions/:version/current", rollbackRoles, setCurrentVolumeVersion)
	g.DELETE("/volumes/:id", rollbackRoles, func(c *gin.Context) {
		deleteVolume(c, store)
	})
	g.POST("/volumes/:id/restore", rollbackRoles, func(c *gin.Context) {
		restoreVolume(c, store)
	})
}

// Soft-delete a volume - admin only.
//
//	@Summary		Delete a volume
//	@Description	Soft-deletes a volume, hiding it from list/browse/picker reads without removing it.
//	@Tags			volumes
//	@Param			id		path		string			true	"Volume ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/volumes/{id} [delete]
func deleteVolume(c *gin.Context, store persistence.CacheStore) {
	id := c.Param("id")
	logging.Logger.Debug("deleteVolume: enter", "id", id)

	existing, err := data.GetVolume(c.Request.Context(), id)
	if err != nil {
		logging.Logger.Error("deleteVolume: volume lookup failed", "id", id, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		logging.Logger.Debug("deleteVolume: exit", "id", id, "outcome", "not_found")
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}

	if err := data.SoftDeleteVolume(c.Request.Context(), id, authz.Subject(c)); err != nil {
		logging.Logger.Error("deleteVolume: soft delete failed", "id", id, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if volumeEventPublisher != nil {
		volumeEventPublisher.PublishEntityDeleted(c.Request.Context(), "volume", id)
	}

	logging.Logger.Info("deleteVolume: exit", "id", id, "outcome", "ok")
	invalidateCachedPaths(store, "/volumes", "/volumes/"+id)
	c.Status(http.StatusNoContent)
}

// Restore a soft-deleted volume - admin only.
//
//	@Summary		Restore a volume
//	@Description	Clears a soft-deleted volume's deletion, returning it to normal read paths.
//	@Tags			volumes
//	@Produce		json
//	@Param			id		path		string			true	"Volume ID"
//	@Success		200		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/volumes/{id}/restore [post]
func restoreVolume(c *gin.Context, store persistence.CacheStore) {
	id := c.Param("id")
	logging.Logger.Debug("restoreVolume: enter", "id", id)

	existing, err := data.GetVolume(c.Request.Context(), id)
	if err != nil {
		logging.Logger.Error("restoreVolume: volume lookup failed", "id", id, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		logging.Logger.Debug("restoreVolume: exit", "id", id, "outcome", "not_found")
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}

	if err := data.RestoreVolume(c.Request.Context(), id); err != nil {
		logging.Logger.Error("restoreVolume: restore failed", "id", id, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	invalidateCachedPaths(store, "/volumes", "/volumes/"+id)

	result, err := data.GetVolume(c.Request.Context(), id)
	if err != nil {
		logging.Logger.Error("restoreVolume: post-restore query failed", "id", id, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	logging.Logger.Info("restoreVolume: exit", "id", id, "outcome", "ok")
	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	if err := jsonapi.MarshalPayload(c.Writer, result); err != nil {
		sentry.CaptureException(err)
	}
}

// List volumes.
//
//	@Summary		List volumes
//	@Description	Lists the volumes in the database.
//	@Tags			volumes
//	@Produce		json
//	@Success		200		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/volumes [get]
func listVolumes(c *gin.Context) {
	params := apiutil.GetQueryParams(c.Request.URL.RawQuery)
	logging.Logger.Debug("Listing volumes with params", "params", params)

	span := tracing.BuildSpanWithParams(c.Request.Context(), "volumes", "list-volumes", params)
	vos, err := data.QueryVolumes(c.Request.Context(), params)
	logging.Logger.Debug("Retrieved volumes", "vos", vos)
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

// Search volumes.
//
//	@Summary		Search volumes
//	@Description	Finds volumes whose title contains q (case-insensitive).
//	@Tags			volumes
//	@Produce		json
//	@Param			q		query		string			true	"Search query"
//	@Success		200		{object}	interface{}
//	@Failure		400		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/volumes/search [get]
func searchVolumes(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}

	_, span := otel.Tracer("volumes").Start(c.Request.Context(), "search-volumes", oteltrace.WithAttributes(attribute.String("q", q)))
	vos, err := data.SearchVolumes(c.Request.Context(), q)
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

// Get a volume.
//
//	@Summary		Get a volume
//	@Description	Get the details of a volume from the database.
//	@Tags			volumes
//	@Produce		json
//	@Param			id		path		string			true	"Volume ID"
//	@Success		204		{object}	interface{}
//	@Failure		404		{object}	interface{}
//	@Failure		500		{object}	interface{}
//	@Router			/volumes/{id} [get]
func getVolume(c *gin.Context) {
	id := c.Param("id")

	_, span := otel.Tracer("volumes").Start(c.Request.Context(), "get-volume", oteltrace.WithAttributes(attribute.String("id", id)))
	vo, err := data.GetVolume(c.Request.Context(), id)
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
