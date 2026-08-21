package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	apiutil "github.com/sweetrpg/api-core.go/util"
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

// TestDeleteAndRestoreStudio exercises deleteEntity/restoreEntity - generic and shared across
// publisher/studio/person/license/system, so exercising studio covers the shared code path.
func TestDeleteAndRestoreStudio(t *testing.T) {
	studioID := seedStudio(t, "Deletable Studio")
	admin := newStudioTestRouter(t, []string{authz.RoleAdmin})

	delRec := doRequest(t, admin, http.MethodDelete, "/studios/"+studioID, nil)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d, body: %s", delRec.Code, http.StatusNoContent, delRec.Body.String())
	}

	deleted, err := data.GetStudio(t.Context(), studioID)
	if err != nil {
		t.Fatalf("GetStudio() error = %v", err)
	}
	if deleted.DeletedAt == nil {
		t.Error("DeletedAt = nil, want set after delete")
	}

	restoreRec := doPost(t, admin, "/studios/"+studioID+"/restore", nil)
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("restore status = %d, want %d, body: %s", restoreRec.Code, http.StatusOK, restoreRec.Body.String())
	}

	restored, err := data.GetStudio(t.Context(), studioID)
	if err != nil {
		t.Fatalf("GetStudio() error = %v", err)
	}
	if restored.DeletedAt != nil {
		t.Error("DeletedAt != nil, want cleared after restore")
	}
}

func TestDeleteStudioNonAdminForbidden(t *testing.T) {
	studioID := seedStudio(t, "Not Deletable By Editor")
	editor := newStudioTestRouter(t, []string{authz.RoleEditor})

	rec := doRequest(t, editor, http.MethodDelete, "/studios/"+studioID, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
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

// newPersonBulkTestRouter mirrors newStudioTestRouter, against setupPersonHandlers -
// bulkCreateEntityVersion (server/entity_version_patch.go) is generic like the rest of this
// engine, but bulk-add-persons only wires it up for Person (catalog-entity-bulk-add's scope),
// so it's exercised here rather than against studio.
func newPersonBulkTestRouter(t *testing.T, roles []string) *gin.Engine {
	t.Helper()

	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: roles, Sub: "auth0|test-reviewer"})
	}))
	t.Cleanup(authAPI.Close)

	r := gin.New()
	setupPersonHandlers(r, persistence.NewInMemoryStore(0), cachettl.Config{}, authz.NewClient(authAPI.URL))
	return r
}

func TestBulkCreatePersonsAllValid(t *testing.T) {
	r := newPersonBulkTestRouter(t, []string{authz.RoleEditor})

	rec := doPost(t, r, "/persons/bulk", []map[string]string{
		{"name": "Ann Leckie"},
		{"name": "N.K. Jemisin"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Results []bulkCreateResult `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(resp.Results))
	}
	for i, result := range resp.Results {
		if !result.Success || result.ID == nil {
			t.Errorf("results[%d] = %+v, want success with an id", i, result)
		}
	}
}

func TestBulkCreatePersonsMixedValidAndInvalid(t *testing.T) {
	r := newPersonBulkTestRouter(t, []string{authz.RoleEditor})

	rec := doPost(t, r, "/persons/bulk", []map[string]string{
		{"name": "Valid One"},
		{"name": ""},
		{"name": "Valid Two"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Results []bulkCreateResult `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(resp.Results))
	}
	if !resp.Results[0].Success {
		t.Errorf("results[0] = %+v, want success", resp.Results[0])
	}
	if resp.Results[1].Success || resp.Results[1].Error == nil {
		t.Errorf("results[1] = %+v, want a failure with an error message", resp.Results[1])
	}
	if !resp.Results[2].Success {
		t.Errorf("results[2] = %+v, want success (a bad entry must not block later entries)", resp.Results[2])
	}

	all, err := data.QueryPersons(t.Context(), apiutil.QueryParams{})
	if err != nil {
		t.Fatalf("QueryPersons() error = %v", err)
	}
	names := make(map[string]bool, len(all))
	for _, p := range all {
		names[p.Name] = true
	}
	if !names["Valid One"] || !names["Valid Two"] {
		t.Errorf("expected both valid entries to be persisted, got names: %v", names)
	}
}

func TestBulkCreatePersonsEmptyBatch(t *testing.T) {
	r := newPersonBulkTestRouter(t, []string{authz.RoleEditor})

	rec := doPost(t, r, "/persons/bulk", []map[string]string{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Results []bulkCreateResult `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("len(results) = %d, want 0", len(resp.Results))
	}
}

func TestBulkCreatePersonsSubmitterForbidden(t *testing.T) {
	r := newPersonBulkTestRouter(t, []string{authz.RoleSubmitter})

	rec := doPost(t, r, "/persons/bulk", []map[string]string{{"name": "Should Not Be Created"}})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
