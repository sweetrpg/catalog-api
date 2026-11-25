package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/internal/events"
)

func TestSetupHandlersRoutesGetPublishBudget(t *testing.T) {
	r := gin.New()
	budgeted := false
	r.Use(func(c *gin.Context) {
		c.Next()
		// Later middleware replaces c.Request, so its context is visible after Next.
		budgeted = events.HasPublishBudget(c.Request.Context())
	})
	SetupHandlers(r, nil, cachettl.Config{}, nil, nil, nil, nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/status/ping", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	if !budgeted {
		t.Error("route registered by SetupHandlers has no publish budget")
	}
}
