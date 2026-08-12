package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/proposedchanges"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/constants"
	"github.com/sweetrpg/mongodb.go/database"
)

func TestMain(m *testing.M) {
	if os.Getenv("TEST_DB_URI") == "" {
		os.Exit(0)
	}
	_ = os.Setenv(constants.DB_URI, os.Getenv("TEST_DB_URI"))
	logging.Init()
	database.SetupDatabase()
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// newTestRouter wires the real setupVolumeHandlers against a fake auth-api that always returns
// roles, so tests exercise the real authz middleware + handler chain end to end.
func newTestRouter(t *testing.T, roles []string) *gin.Engine {
	t.Helper()

	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: roles, Sub: "auth0|test-reviewer"})
	}))
	t.Cleanup(authAPI.Close)

	r := gin.New()
	setupVolumeHandlers(r, persistence.NewInMemoryStore(0), cachettl.Config{}, authz.NewClient(authAPI.URL))
	return r
}

func seedVolume(t *testing.T, title string) *vo.VolumeVO {
	t.Helper()

	id, err := data.AddVolume(t.Context(), &vo.VolumeVO{Title: title, Description: "seed description"})
	if err != nil {
		t.Fatalf("AddVolume() error = %v", err)
	}
	got, err := data.GetVolume(t.Context(), *id)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	return got
}

// seedPublisher/seedStudio insert a minimal Publisher/Studio document directly - catalog-data.go
// has no Add function for either (read-only entities, per AGENTS.md), so tests that need one to
// exist go straight through the mongodb.go database package instead of the data package.
func seedPublisher(t *testing.T, id, name string) {
	t.Helper()
	if _, err := database.Insert("publishers", catalogmodels.Publisher{ID: id, Name: name}); err != nil {
		t.Fatalf("seed publisher: %v", err)
	}
}

func seedStudio(t *testing.T, id, name string) {
	t.Helper()
	if _, err := database.Insert("studios", catalogmodels.Studio{ID: id, Name: name}); err != nil {
		t.Fatalf("seed studio: %v", err)
	}
}

func seedPerson(t *testing.T, id, name string) {
	t.Helper()
	if _, err := database.Insert("persons", catalogmodels.Person{ID: id, Name: name}); err != nil {
		t.Fatalf("seed person: %v", err)
	}
}

func doPatch(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer some-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func doPost(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Authorization", "Bearer some-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPatchVolumeEditorAppliesDirectly(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleEditor})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]string{"title": "Edited By Editor"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Edited By Editor" {
		t.Errorf("live Title = %q, want %q", got.Title, "Edited By Editor")
	}
}

func TestPatchVolumeEditorAppliesProperties(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleEditor})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]any{
		"properties": []map[string]string{
			{"name": "ISBN", "kind": "text", "value": "978-0-000-00000-0"},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if len(got.Properties) != 1 || got.Properties[0].Name != "ISBN" || got.Properties[0].Value != "978-0-000-00000-0" {
		t.Errorf("Properties = %+v, want one ISBN property", got.Properties)
	}
}

func TestPatchVolumeEditorAppliesPublisherAndStudioIDs(t *testing.T) {
	seedPublisher(t, "pub-1", "Test Publisher")
	seedStudio(t, "studio-1", "Test Studio")
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleEditor})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]any{
		"publisherIds": []string{"pub-1"},
		"studioIds":    []string{"studio-1"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if len(got.Publishers) != 1 || got.Publishers[0].ID != "pub-1" {
		t.Errorf("Publishers = %+v, want [pub-1]", got.Publishers)
	}
	if len(got.Studios) != 1 || got.Studios[0].ID != "studio-1" {
		t.Errorf("Studios = %+v, want [studio-1]", got.Studios)
	}
}

func TestPatchVolumeEditorAppliesFormat(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleEditor})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]any{"format": "hardcover"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Format != "hardcover" {
		t.Errorf("Format = %q, want %q", got.Format, "hardcover")
	}
}

func TestPatchVolumeEditorAppliesCredits(t *testing.T) {
	seedPerson(t, "person-1", "Test Author")
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleEditor})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]any{
		"credits": []map[string]string{{"personId": "person-1", "contributionType": "Author"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	credits, err := data.QueryContributionsByVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("QueryContributionsByVolume() error = %v", err)
	}
	if len(credits) != 1 || credits[0].Person.ID != "person-1" || len(credits[0].Roles) != 1 || credits[0].Roles[0] != "Author" {
		t.Fatalf("credits = %+v, want one person-1/Author credit", credits)
	}

	// A second PATCH with an empty credits list removes the previously added credit.
	rec = doPatch(t, r, "/volumes/"+seed.ID, map[string]any{"credits": []map[string]string{}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	credits, err = data.QueryContributionsByVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("QueryContributionsByVolume() error = %v", err)
	}
	if len(credits) != 0 {
		t.Errorf("credits after removal = %+v, want none", credits)
	}
}

func TestPatchVolumeSubmitterCannotSetFormatOrCredits(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleSubmitter})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]any{"format": "PDF"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("format status = %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	rec = doPatch(t, r, "/volumes/"+seed.ID, map[string]any{
		"credits": []map[string]string{{"personId": "person-1", "contributionType": "Author"}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("credits status = %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPatchVolumeSubmitterCannotSetEditorOnlyFields(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleSubmitter})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]any{
		"title":      "Should Be Rejected Entirely",
		"properties": []map[string]string{{"name": "ISBN", "kind": "text", "value": "123"}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Original Title" {
		t.Errorf("live Title = %q, want unchanged - the whole request should be rejected, not partially applied", got.Title)
	}
	pending, err := proposedchanges.ListPending(t.Context(), "volume", seed.ID)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("ListPending() returned %d, want 0 - no proposal should be created either", len(pending))
	}
}

func TestPatchVolumeSubmitterCreatesProposalWithoutTouchingLiveRecord(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleSubmitter})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]string{"title": "Proposed By Submitter"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Original Title" {
		t.Errorf("live Title = %q, want unchanged %q", got.Title, "Original Title")
	}

	pending, err := proposedchanges.ListPending(t.Context(), "volume", seed.ID)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("ListPending() returned %d, want 1", len(pending))
	}
	if pending[0].Diff["title"].New != "Proposed By Submitter" {
		t.Errorf("proposal title = %v, want %q", pending[0].Diff["title"].New, "Proposed By Submitter")
	}
}

func TestPatchVolumeUserRoleForbidden(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleUser})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]string{"title": "Should Not Apply"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestListAndAcceptAllProposedChanges(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{
		"title":       "Accepted Title",
		"description": "Accepted Description",
	})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	listRec := httptest.NewRequest(http.MethodGet, "/volumes/"+seed.ID+"/proposed-changes", nil)
	listRec.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	editor.ServeHTTP(rec, listRec)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", rec.Code, http.StatusOK)
	}
	var proposals []proposedchanges.ProposedChange
	if err := json.Unmarshal(rec.Body.Bytes(), &proposals); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(proposals) != 1 {
		t.Fatalf("listed %d proposals, want 1", len(proposals))
	}
	proposalID := proposals[0].ID.Hex()

	acceptRec := doPost(t, editor, "/volumes/"+seed.ID+"/proposed-changes/"+proposalID+"/accept", nil)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept status = %d, want %d, body: %s", acceptRec.Code, http.StatusOK, acceptRec.Body.String())
	}
	var acceptResp reviewProposalResponse
	if err := json.Unmarshal(acceptRec.Body.Bytes(), &acceptResp); err != nil {
		t.Fatalf("unmarshal accept response: %v", err)
	}
	if acceptResp.Status != proposedchanges.StatusAccepted {
		t.Errorf("Status = %q, want %q", acceptResp.Status, proposedchanges.StatusAccepted)
	}
	if len(acceptResp.Applied) != 2 {
		t.Errorf("Applied = %v, want 2 fields", acceptResp.Applied)
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Accepted Title" || got.Description != "Accepted Description" {
		t.Errorf("live volume = %+v, want title/description applied", got)
	}
}

func TestAcceptSubsetOfFields(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{
		"title":       "New Title",
		"description": "New Description",
	})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	pending, _ := proposedchanges.ListPending(t.Context(), "volume", seed.ID)
	proposalID := pending[0].ID.Hex()

	rec := doPost(t, editor, "/volumes/"+seed.ID+"/proposed-changes/"+proposalID+"/accept", map[string][]string{
		"fields": {"title"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp reviewProposalResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != proposedchanges.StatusPartiallyAccepted {
		t.Errorf("Status = %q, want %q", resp.Status, proposedchanges.StatusPartiallyAccepted)
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "New Title" {
		t.Errorf("Title = %q, want %q applied", got.Title, "New Title")
	}
	if got.Description != "seed description" {
		t.Errorf("Description = %q, want unchanged", got.Description)
	}
}

func TestRejectProposedChange(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{"title": "Rejected Title"})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	pending, _ := proposedchanges.ListPending(t.Context(), "volume", seed.ID)
	proposalID := pending[0].ID.Hex()

	rec := doPost(t, editor, "/volumes/"+seed.ID+"/proposed-changes/"+proposalID+"/reject", map[string]string{
		"note": "not needed",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Original Title" {
		t.Errorf("live Title = %q, want unchanged", got.Title)
	}

	proposal, err := proposedchanges.Get(t.Context(), proposalID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if proposal.Status != proposedchanges.StatusRejected {
		t.Errorf("Status = %q, want %q", proposal.Status, proposedchanges.StatusRejected)
	}
	if proposal.ReviewNote != "not needed" {
		t.Errorf("ReviewNote = %q, want %q", proposal.ReviewNote, "not needed")
	}
}

func TestReviewingAlreadyReviewedProposalReturns409(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{"title": "First"})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	pending, _ := proposedchanges.ListPending(t.Context(), "volume", seed.ID)
	proposalID := pending[0].ID.Hex()

	first := doPost(t, editor, "/volumes/"+seed.ID+"/proposed-changes/"+proposalID+"/reject", nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first reject status = %d, want %d", first.Code, http.StatusOK)
	}

	second := doPost(t, editor, "/volumes/"+seed.ID+"/proposed-changes/"+proposalID+"/accept", nil)
	if second.Code != http.StatusConflict {
		t.Fatalf("second review status = %d, want %d", second.Code, http.StatusConflict)
	}
}

func TestAcceptFlagsConflictWithoutApplyingStaleValue(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{"title": "Submitter's Title"})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	// A direct edit lands after the proposal was submitted, changing the live title out from
	// under it.
	doPatch(t, editor, "/volumes/"+seed.ID, map[string]string{"title": "Editor's Direct Edit"})

	pending, _ := proposedchanges.ListPending(t.Context(), "volume", seed.ID)
	proposalID := pending[0].ID.Hex()

	rec := doPost(t, editor, "/volumes/"+seed.ID+"/proposed-changes/"+proposalID+"/accept", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp reviewProposalResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Conflicts) != 1 || resp.Conflicts[0] != "title" {
		t.Errorf("Conflicts = %v, want [title]", resp.Conflicts)
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Editor's Direct Edit" {
		t.Errorf("Title = %q, want editor's direct edit to survive, not the stale proposal value", got.Title)
	}
}
