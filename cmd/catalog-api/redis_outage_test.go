package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-contrib/cache"
	"github.com/gin-gonic/gin"
	apiconstants "github.com/sweetrpg/api-core.go/constants"
)

// stalledRedis accepts connections (via the kernel backlog) and never replies.
func stalledRedis(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestCachedRouteServesUncachedWhenRedisUnreachable(t *testing.T) {
	refused := stalledRedis(t)
	_ = refused.Close()

	for name, l := range map[string]net.Listener{"refused": refused, "stalled": stalledRedis(t)} {
		t.Run(name, func(t *testing.T) {
			host, port, err := net.SplitHostPort(l.Addr().String())
			if err != nil {
				t.Fatalf("split address: %v", err)
			}
			t.Setenv(apiconstants.REDIS_HOST, host)
			t.Setenv(apiconstants.REDIS_PORT, port)

			pool := setupRedisPool()
			t.Cleanup(func() { _ = pool.Close() })
			store := setupCache(pool)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(cacheInvalidationMiddleware(store, pool))
			handler := cache.CachePage(store, time.Minute, func(c *gin.Context) { c.String(http.StatusOK, "fresh") })
			r.GET("/volumes", handler)
			r.PATCH("/volumes", func(c *gin.Context) { c.Status(http.StatusOK) })

			for _, method := range []string{http.MethodGet, http.MethodPatch} {
				rec := httptest.NewRecorder()
				start := time.Now()
				r.ServeHTTP(rec, httptest.NewRequest(method, "/volumes", nil))

				if rec.Code != http.StatusOK {
					t.Fatalf("%s status = %d, want 200", method, rec.Code)
				}
				if method == http.MethodGet && rec.Body.String() != "fresh" {
					t.Fatalf("body = %q, want %q", rec.Body.String(), "fresh")
				}
				if elapsed := time.Since(start); elapsed > 4*time.Second {
					t.Fatalf("%s took %s, want the cache to give up quickly", method, elapsed)
				}
			}
		})
	}
}
