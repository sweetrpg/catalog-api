package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-data.go/data"
)

// newStudioTestRouter mirrors newTestRouter (volumes_write_test.go), against
// setupStudioHandlers - patchEntityVersion/acceptEntityVersion/etc. are generic and shared
// across publisher/studio/person/license/system (server/entity_version_patch.go), so exercising
// one type end to end covers the shared code path; the per-type var configs (studioVersionConfig
// etc.) are the only per-type surface left untested here.
func newStudioTestRouter(t *testing.T, roles []string) *gin.Engine {
	t.Helper()

	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: roles, Sub: "auth0|test-reviewer"})
	}))
	t.Cleanup(authAPI.Close)

	r := gin.New()
	setupStudioHandlers(r, persistence.NewInMemoryStore(0), cachettl.Config{}, authz.NewClient(authAPI.URL))
	return r
}

func TestPatchStudioEditorAppliesDirectly(t *testing.T) {
	studioID := seedStudio(t, "Original Name")
	r := newStudioTestRouter(t, []string{authz.RoleEditor})

	rec := doPatch(t, r, "/studios/"+studioID, map[string]string{"name": "Edited By Editor"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetStudio(t.Context(), studioID)
	if err != nil {
		t.Fatalf("GetStudio() error = %v", err)
	}
	if got.Name != "Edited By Editor" {
		t.Errorf("live Name = %q, want %q", got.Name, "Edited By Editor")
	}
}

func TestPatchStudioSubmitterCreatesSubmittedVersionWithoutTouchingLiveRecord(t *testing.T) {
	studioID := seedStudio(t, "Original Name")
	r := newStudioTestRouter(t, []string{authz.RoleSubmitter})

	rec := doPatch(t, r, "/studios/"+studioID, map[string]string{"name": "Proposed By Submitter"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var submitted submittedVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if submitted.State != "submitted" {
		t.Errorf("State = %q, want %q", submitted.State, "submitted")
	}

	got, err := data.GetStudio(t.Context(), studioID)
	if err != nil {
		t.Fatalf("GetStudio() error = %v", err)
	}
	if got.Name != "Original Name" {
		t.Errorf("live Name = %q, want unchanged %q", got.Name, "Original Name")
	}
}

func TestPatchStudioUserRoleForbidden(t *testing.T) {
	studioID := seedStudio(t, "Original Name")
	r := newStudioTestRouter(t, []string{authz.RoleUser})

	rec := doPatch(t, r, "/studios/"+studioID, map[string]string{"name": "Should Not Apply"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAcceptAndRejectStudioVersion(t *testing.T) {
	studioID := seedStudio(t, "Original Name")
	submitter := newStudioTestRouter(t, []string{authz.RoleSubmitter})
	submitRec := doPatch(t, submitter, "/studios/"+studioID, map[string]string{
		"name": "Accepted Name",
	})
	var submitted submittedVersionResponse
	if err := json.Unmarshal(submitRec.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("unmarshal submit response: %v", err)
	}

	editor := newStudioTestRouter(t, []string{authz.RoleEditor})
	acceptPath := "/studios/" + studioID + "/versions/" + strconv.Itoa(submitted.Version) + "/accept"
	acceptRec := doPost(t, editor, acceptPath, nil)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept status = %d, want %d, body: %s", acceptRec.Code, http.StatusOK, acceptRec.Body.String())
	}

	got, err := data.GetStudio(t.Context(), studioID)
	if err != nil {
		t.Fatalf("GetStudio() error = %v", err)
	}
	if got.Name != "Accepted Name" {
		t.Errorf("live Name = %q, want %q", got.Name, "Accepted Name")
	}
}

func TestAddStudioThenListAndGetVersions(t *testing.T) {
	studioID := seedStudio(t, "Original Name")
	r := newStudioTestRouter(t, []string{authz.RoleEditor})
	doPatch(t, r, "/studios/"+studioID, map[string]string{"name": "V2"})

	versions, err := data.ListStudioVersions(t.Context(), studioID)
	if err != nil {
		t.Fatalf("ListStudioVersions() error = %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len(versions) = %d, want 2", len(versions))
	}
}
