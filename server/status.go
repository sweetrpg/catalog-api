package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	apicores "github.com/sweetrpg/api-core.go/server"
	"github.com/sweetrpg/api-core.go/vo"
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

	// Run the cache check concurrently with api-core.go's Mongo checks rather than after them -
	// health callers allow 10s, matching api-core.go's Mongo budget
	// alone; stacking the cache ping on top sequentially could push total handler time past
	// that window and fail the probe on latency even when every individual check would have
	// succeeded.
	cacheCtx, cacheCancel := context.WithTimeout(c.Request.Context(), cacheReadyTimeout)
	defer cacheCancel()
	cacheReadyCh := make(chan bool, 1)
	go func() { cacheReadyCh <- readiness.CacheReady(cacheCtx) }()

	resp := apicores.HealthHandler(c.Request.Context())

	noteCacheStatus(&resp, <-cacheReadyCh)

	status := http.StatusOK
	if resp.Errors > 0 {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, resp)
}

// noteCacheStatus reports an unreachable cache without counting it as an error: the service
// serves uncached, so a Redis outage must not fail health.
func noteCacheStatus(resp *vo.HealthResponseVO, ready bool) {
	if !ready {
		resp.Messages = append(resp.Messages, "cache backend unreachable")
	}
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
