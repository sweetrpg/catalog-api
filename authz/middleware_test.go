package authz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/common.go/logging"
)

func init() {
	gin.SetMode(gin.TestMode)
	logging.Init()
}

func newTestRouter(t *testing.T, authAPIRoles []string, authAPIAllowed bool, allowedRoles ...string) *gin.Engine {
	t.Helper()

	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(CheckResponse{Allowed: authAPIAllowed, Roles: authAPIRoles, Sub: "auth0|test"})
	}))
	t.Cleanup(authAPI.Close)

	client := NewClient(authAPI.URL)

	r := gin.New()
	r.PATCH("/volumes/:id", RequireAnyRole(client, "catalog-api", allowedRoles...), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"roles": Roles(c), "sub": Subject(c)})
	})
	return r
}

func TestRequireAnyRoleMissingTokenReturns401(t *testing.T) {
	r := newTestRouter(t, []string{RoleUser}, true, RoleEditor, RoleAdmin, RoleSubmitter)

	req := httptest.NewRequest(http.MethodPatch, "/volumes/abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAnyRoleUserOnlyReturns403(t *testing.T) {
	r := newTestRouter(t, []string{RoleUser}, true, RoleEditor, RoleAdmin, RoleSubmitter)

	req := httptest.NewRequest(http.MethodPatch, "/volumes/abc", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireAnyRoleQualifyingRolePasses(t *testing.T) {
	r := newTestRouter(t, []string{RoleEditor}, true, RoleEditor, RoleAdmin, RoleSubmitter)

	req := httptest.NewRequest(http.MethodPatch, "/volumes/abc", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestRequireAnyRoleServiceDeniedReturns403(t *testing.T) {
	r := newTestRouter(t, []string{RoleEditor}, false, RoleEditor, RoleAdmin, RoleSubmitter)

	req := httptest.NewRequest(http.MethodPatch, "/volumes/abc", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireAnyRoleBackendUnavailableReturns503(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	r := gin.New()
	r.PATCH("/volumes/:id", RequireAnyRole(client, "catalog-api", RoleEditor, RoleAdmin), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	req := httptest.NewRequest(http.MethodPatch, "/volumes/abc", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
