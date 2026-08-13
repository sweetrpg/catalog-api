package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/proposedchanges"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
)

// newPublisherTestRouter mirrors newTestRouter (volumes_write_test.go), against
// setupPublisherHandlers - patchEntity/acceptEntityProposedChange/etc. are generic and shared
// across all four entity types, so exercising one type end to end covers the shared code path;
// the per-type var configs (publisherPatchConfig etc.) are the only per-type surface left
// untested here.
func newPublisherTestRouter(t *testing.T, roles []string) *gin.Engine {
	t.Helper()

	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: roles, Sub: "auth0|test-reviewer"})
	}))
	t.Cleanup(authAPI.Close)

	r := gin.New()
	setupPublisherHandlers(r, persistence.NewInMemoryStore(0), cachettl.Config{}, authz.NewClient(authAPI.URL))
	return r
}

func seedPublisher(t *testing.T, id, name string) {
	t.Helper()

	_, err := database.Insert[models.Publisher]("publishers", models.Publisher{ID: id, Name: name})
	if err != nil {
		t.Fatalf("seed publisher: %v", err)
	}
	t.Cleanup(func() {
		// context.Background(), not t.Context() - the latter is already canceled by the time
		// Cleanup funcs run, which would make this delete silently no-op.
		_, _ = database.Db.Collection("publishers").DeleteMany(context.Background(), bson.D{{Key: "_id", Value: id}})
	})
}

func TestPatchPublisherEditorAppliesDirectly(t *testing.T) {
	seedPublisher(t, "editor-publisher", "Original Name")
	r := newPublisherTestRouter(t, []string{authz.RoleEditor})

	rec := doPatch(t, r, "/publishers/editor-publisher", map[string]string{"name": "Edited By Editor"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetPublisher(t.Context(), "editor-publisher")
	if err != nil {
		t.Fatalf("GetPublisher() error = %v", err)
	}
	if got.Name != "Edited By Editor" {
		t.Errorf("live Name = %q, want %q", got.Name, "Edited By Editor")
	}
}

func TestPatchPublisherSubmitterCreatesProposalWithoutTouchingLiveRecord(t *testing.T) {
	seedPublisher(t, "submitter-publisher", "Original Name")
	r := newPublisherTestRouter(t, []string{authz.RoleSubmitter})

	rec := doPatch(t, r, "/publishers/submitter-publisher", map[string]string{"name": "Proposed By Submitter"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	got, err := data.GetPublisher(t.Context(), "submitter-publisher")
	if err != nil {
		t.Fatalf("GetPublisher() error = %v", err)
	}
	if got.Name != "Original Name" {
		t.Errorf("live Name = %q, want unchanged %q", got.Name, "Original Name")
	}

	pending, err := proposedchanges.ListPending(t.Context(), "publisher", "submitter-publisher")
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("ListPending() returned %d, want 1", len(pending))
	}
	if pending[0].Diff["name"].New != "Proposed By Submitter" {
		t.Errorf("proposal name = %v, want %q", pending[0].Diff["name"].New, "Proposed By Submitter")
	}
}

func TestPatchPublisherUserRoleForbidden(t *testing.T) {
	seedPublisher(t, "forbidden-publisher", "Original Name")
	r := newPublisherTestRouter(t, []string{authz.RoleUser})

	rec := doPatch(t, r, "/publishers/forbidden-publisher", map[string]string{"name": "Should Not Apply"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAcceptAndRejectPublisherProposedChange(t *testing.T) {
	seedPublisher(t, "review-publisher", "Original Name")
	submitter := newPublisherTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/publishers/review-publisher", map[string]string{
		"name":    "Accepted Name",
		"address": "Rejected Address",
	})

	editor := newPublisherTestRouter(t, []string{authz.RoleEditor})
	listReq := httptest.NewRequest(http.MethodGet, "/publishers/review-publisher/proposed-changes", nil)
	listReq.Header.Set("Authorization", "Bearer some-token")
	listRec := httptest.NewRecorder()
	editor.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}
	var proposals []proposedchanges.ProposedChange
	if err := json.Unmarshal(listRec.Body.Bytes(), &proposals); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(proposals) != 1 {
		t.Fatalf("listed %d proposals, want 1", len(proposals))
	}
	proposalID := proposals[0].ID.Hex()

	acceptRec := doPost(t, editor, "/publishers/review-publisher/proposed-changes/"+proposalID+"/accept",
		map[string]any{"fields": []string{"name"}})
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept status = %d, want %d, body: %s", acceptRec.Code, http.StatusOK, acceptRec.Body.String())
	}

	got, err := data.GetPublisher(t.Context(), "review-publisher")
	if err != nil {
		t.Fatalf("GetPublisher() error = %v", err)
	}
	if got.Name != "Accepted Name" {
		t.Errorf("live Name = %q, want %q", got.Name, "Accepted Name")
	}
	if got.Address != "" {
		t.Errorf("live Address = %q, want unchanged empty", got.Address)
	}
}
