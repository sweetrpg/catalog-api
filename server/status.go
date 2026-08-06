package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	apicores "github.com/sweetrpg/api-core.go/server"
	_ "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/readiness"
	"github.com/sweetrpg/common.go/logging"
)

// cacheReadyTimeout bounds the live Redis ping in healthHandler, matching api-core.go's
// healthCheckTimeout for the equivalent Mongo checks.
const cacheReadyTimeout = 5 * time.Second

func setupStatusHandlers(g *gin.Engine) {
	logging.Logger.Info("Setting up status endpoint handlers...")

	{
		a := g.Group("/status")
		// TODO: a.Use()
		a.GET("/health", healthHandler)
	}

	g.GET("/status/ping", pingHandler)
}

// Health check.
//
//	 Do a health check of the application and its dependencies and return the results
//		@Summary		Health check
//		@Description	Health check
//		@Tags			status
//		@Produce		json
//		@Success		200		{object}	vo.HealthResponseVO
//		@Router			/status/health [get]
func healthHandler(c *gin.Context) {
	// authHeader := c.Request.Header["Authorization"]
	// if authHeader == nil || len(authHeader) != 1 {
	// 	c.JSON(http.StatusUnauthorized, apicorev.ErrorVO{
	// 		Error: apicorec.ErrorUnauthorized,
	// 	})
	// 	return
	// }
	// if authHeader[0] != os.Getenv(constants.HEALTH_TOKEN) {
	// 	c.JSON(http.StatusForbidden, apicorev.ErrorVO{
	// 		Error: apicorec.ErrorForbidden,
	// 	})
	// 	return
	// }

	resp := apicores.HealthHandler(c.Request.Context())

	// Fold the cache backend's reachability into the same readiness signal Mongo already
	// reports, so a configured-but-unreachable Redis surfaces as a failing readiness probe
	// (Degraded in ArgoCD) instead of a silent cache-miss-only degradation. Live-pinged on
	// every call rather than trusting a boot-time snapshot, so a Redis outage that resolves
	// lets readiness recover without a pod restart.
	cacheCtx, cacheCancel := context.WithTimeout(c.Request.Context(), cacheReadyTimeout)
	defer cacheCancel()
	if !readiness.CacheReady(cacheCtx) {
		resp.Messages = append(resp.Messages, "cache backend unreachable")
		resp.Errors++
	}

	status := http.StatusOK
	if resp.Errors > 0 {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, resp)
}

// Ping the application.
//
//	 A shallow health check to see if basic responses are working. Returns the date and hostname of the server handling the request.
//		@Summary		Ping
//		@Description	Ping
//		@Tags			status
//		@Produce		json
//		@Success		200		{object}	vo.PingResponseVO
//		@Router			/status/ping [get]
func pingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, apicores.PingHandler())
}
