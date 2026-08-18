package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-data.go/data"
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

// testDeps exposes the fakes newTestRouter wires up, so individual tests can seed a staged
// asset or an edit session directly (bypassing the endpoints that would normally create them -
// catalog-web writes sessions and stages assets, neither of which catalog-api itself does).
type testDeps struct {
	Router    *gin.Engine
	AssetsURL string
	RedisPool *redis.Pool
}

// newTestRouter wires the real setupVolumeHandlers against a fake auth-api that always returns
// roles, a fake assets-web, and a real editsession.Store backed by miniredis, so tests exercise
// the real authz middleware + handler chain end to end.
func newTestRouter(t *testing.T, roles []string) *gin.Engine {
	t.Helper()
	return newTestDeps(t, roles).Router
}

func newTestDeps(t *testing.T, roles []string) testDeps {
	t.Helper()
	return newTestDepsWithAssets(t, roles, newFakeAssetsServer(t).URL)
}

// newTestDepsWithAssets is newTestDeps but against a caller-supplied assets-web fake, so two
// routers (e.g. a submitter's and a reviewing editor's) can share one staged-asset store - a
// proposal's staged cover/samples must be visible to whichever router later accepts/rejects it.
func newTestDepsWithAssets(t *testing.T, roles []string, assetsURL string) testDeps {
	t.Helper()

	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: roles, Sub: "auth0|test-reviewer"})
	}))
	t.Cleanup(authAPI.Close)

	redisPool := newTestRedisPool(t)
	authzClient := authz.NewClient(authAPI.URL)

	r := gin.New()
	setupVolumeHandlers(r, persistence.NewInMemoryStore(0), cachettl.Config{}, authzClient, assets.NewClient(assetsURL), editsession.NewStore(redisPool))
	setupSubmissionCapHandlers(r, authzClient)
	return testDeps{Router: r, AssetsURL: assetsURL, RedisPool: redisPool}
}

// seedEditSession writes a session directly into the fixture's Redis, in place of catalog-web
// (which owns real session writes - catalog-api only reads/deletes).
func seedEditSession(t *testing.T, deps testDeps, userID, recordType string, session editsession.Session) {
	t.Helper()

	conn, err := deps.RedisPool.GetContext(t.Context())
	if err != nil {
		t.Fatalf("get redis connection: %v", err)
	}
	defer func() { _ = conn.Close() }()

	raw, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	if _, err := conn.Do("SET", editsession.Key(userID, recordType), raw); err != nil {
		t.Fatalf("seed session: %v", err)
	}
}

// seedStagedAsset uploads bytes directly to the fixture's fake assets-web under kind/id, in
// place of a real client-side upload.
func seedStagedAsset(t *testing.T, deps testDeps, kind, id string, data []byte) {
	t.Helper()

	client := assets.NewClient(deps.AssetsURL)
	if err := client.Store(t.Context(), kind, id, data, "image/png"); err != nil {
		t.Fatalf("seed staged asset: %v", err)
	}
}

// newFakeAssetsServer is a minimal in-memory stand-in for assets-web's GET/POST/DELETE
// /asset/<kind>/<id>, sufficient to exercise cover/sample staging-and-promotion end to end
// without a real assets-web instance.
func newFakeAssetsServer(t *testing.T) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	store := map[string][]byte{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path // "/asset/<kind>/<id>"
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			data, ok := store[key]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(data)
		case http.MethodPost:
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer func() { _ = file.Close() }()
			body, err := io.ReadAll(file)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mu.Lock()
			store[key] = body
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			mu.Lock()
			_, ok := store[key]
			delete(store, key)
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestRedisPool(t *testing.T) *redis.Pool {
	t.Helper()

	mr := miniredis.RunT(t)
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", mr.Addr())
		},
	}
	t.Cleanup(func() { _ = pool.Close() })
	return pool
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

// seedPublisher/seedStudio/seedPerson create a record via its Add function (meta+version model,
// task group 7) rather than a raw flat-collection insert - all four generic types moved off the
// flat collections these used to write to directly. Each returns the generated id, since Add
// generates it rather than accepting a caller-supplied one.
func seedPublisher(t *testing.T, name string) string {
	t.Helper()
	id, err := data.AddPublisher(t.Context(), &vo.PublisherVO{Name: name})
	if err != nil {
		t.Fatalf("seed publisher: %v", err)
	}
	return *id
}

func seedStudio(t *testing.T, name string) string {
	t.Helper()
	id, err := data.AddStudio(t.Context(), &vo.StudioVO{Name: name})
	if err != nil {
		t.Fatalf("seed studio: %v", err)
	}
	return *id
}

func seedPerson(t *testing.T, name string) string {
	t.Helper()
	id, err := data.AddPerson(t.Context(), &vo.PersonVO{Name: name})
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return *id
}

func doPatch(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return doRequest(t, r, http.MethodPatch, path, body)
}

// doRequest is the general form doPatch/doPost delegate to - kept for methods (PUT, ...)
// neither of them covers.
func doRequest(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(buf))
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
	publisherID := seedPublisher(t, "Test Publisher")
	studioID := seedStudio(t, "Test Studio")
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleEditor})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]any{
		"publisherIds": []string{publisherID},
		"studioIds":    []string{studioID},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if len(got.Publishers) != 1 || got.Publishers[0].ID != publisherID {
		t.Errorf("Publishers = %+v, want [%s]", got.Publishers, publisherID)
	}
	if len(got.Studios) != 1 || got.Studios[0].ID != studioID {
		t.Errorf("Studios = %+v, want [%s]", got.Studios, studioID)
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
	personID := seedPerson(t, "Test Author")
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleEditor})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]any{
		"credits": []map[string]string{{"personId": personID, "contributionType": "Author"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	credits, err := data.QueryContributionsByVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("QueryContributionsByVolume() error = %v", err)
	}
	if len(credits) != 1 || credits[0].Person.ID != personID || len(credits[0].Roles) != 1 || credits[0].Roles[0] != "Author" {
		t.Fatalf("credits = %+v, want one %s/Author credit", credits, personID)
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
	versions, err := data.ListVolumeVersions(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("ListVolumeVersions() error = %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("ListVolumeVersions() returned %d, want 1 - no submitted version should be created either", len(versions))
	}
}

func TestPatchVolumeSubmitterCreatesSubmittedVersionWithoutTouchingLiveRecord(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	r := newTestRouter(t, []string{authz.RoleSubmitter})

	rec := doPatch(t, r, "/volumes/"+seed.ID, map[string]string{"title": "Proposed By Submitter"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var resp submittedVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Version != 2 || resp.State != "submitted" {
		t.Errorf("response = %+v, want version 2, state submitted", resp)
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Original Title" {
		t.Errorf("live Title = %q, want unchanged %q", got.Title, "Original Title")
	}

	version, err := data.GetVolumeVersion(t.Context(), seed.ID, 2)
	if err != nil {
		t.Fatalf("GetVolumeVersion() error = %v", err)
	}
	if version == nil {
		t.Fatalf("GetVolumeVersion() = nil, want the submitted version")
	}
	if version.Title != "Proposed By Submitter" {
		t.Errorf("submitted version title = %q, want %q", version.Title, "Proposed By Submitter")
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

func TestListAndAcceptAllVersions(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{
		"title":       "Accepted Title",
		"description": "Accepted Description",
	})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	listRec := httptest.NewRequest(http.MethodGet, "/volumes/"+seed.ID+"/versions", nil)
	listRec.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	editor.ServeHTTP(rec, listRec)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", rec.Code, http.StatusOK)
	}
	var versions []vo.VolumeVersionVO
	if err := json.Unmarshal(rec.Body.Bytes(), &versions); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("listed %d versions, want 2", len(versions))
	}

	acceptRec := doPost(t, editor, fmt.Sprintf("/volumes/%s/versions/2/accept", seed.ID), nil)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept status = %d, want %d, body: %s", acceptRec.Code, http.StatusOK, acceptRec.Body.String())
	}
	var acceptResp reviewVersionResponse
	if err := json.Unmarshal(acceptRec.Body.Bytes(), &acceptResp); err != nil {
		t.Fatalf("unmarshal accept response: %v", err)
	}
	if acceptResp.State != "live" {
		t.Errorf("State = %q, want %q", acceptResp.State, "live")
	}
	if len(acceptResp.Conflicts) != 0 {
		t.Errorf("Conflicts = %v, want none", acceptResp.Conflicts)
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Accepted Title" || got.Description != "Accepted Description" {
		t.Errorf("live volume = %+v, want title/description applied", got)
	}
}

func TestAcceptSubsetOfFieldsDerivesNewVersion(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{
		"title":       "New Title",
		"description": "New Description",
	})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	rec := doPost(t, editor, fmt.Sprintf("/volumes/%s/versions/2/accept", seed.ID), map[string][]string{
		"fields": {"title"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp reviewVersionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Version != 3 {
		t.Errorf("Version = %d, want 3 (a derived version)", resp.Version)
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

	original, err := data.GetVolumeVersion(t.Context(), seed.ID, 2)
	if err != nil {
		t.Fatalf("GetVolumeVersion() error = %v", err)
	}
	if original.State != "partially_accepted" {
		t.Errorf("original version state = %q, want %q", original.State, "partially_accepted")
	}
	if original.ResultingVersion == nil || *original.ResultingVersion != 3 {
		t.Errorf("original version ResultingVersion = %v, want pointer to 3", original.ResultingVersion)
	}
}

func TestRejectVolumeVersion(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{"title": "Rejected Title"})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	rec := doPost(t, editor, fmt.Sprintf("/volumes/%s/versions/2/reject", seed.ID), map[string]string{
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

	version, err := data.GetVolumeVersion(t.Context(), seed.ID, 2)
	if err != nil {
		t.Fatalf("GetVolumeVersion() error = %v", err)
	}
	if version.State != "rejected" {
		t.Errorf("State = %q, want %q", version.State, "rejected")
	}
	if version.ReviewNote == nil || *version.ReviewNote != "not needed" {
		t.Errorf("ReviewNote = %v, want %q", version.ReviewNote, "not needed")
	}
}

func TestReviewingAlreadyReviewedVersionReturns400(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{"title": "First"})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	first := doPost(t, editor, fmt.Sprintf("/volumes/%s/versions/2/reject", seed.ID), nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first reject status = %d, want %d", first.Code, http.StatusOK)
	}

	second := doPost(t, editor, fmt.Sprintf("/volumes/%s/versions/2/accept", seed.ID), nil)
	if second.Code != http.StatusBadRequest {
		t.Fatalf("second review status = %d, want %d", second.Code, http.StatusBadRequest)
	}
}

func TestAcceptFlagsConflictWithoutApplyingStaleValue(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	submitter := newTestRouter(t, []string{authz.RoleSubmitter})
	doPatch(t, submitter, "/volumes/"+seed.ID, map[string]string{"title": "Submitter's Title"})

	editor := newTestRouter(t, []string{authz.RoleEditor})
	// A direct edit lands after the submission, changing the live title out from under it.
	doPatch(t, editor, "/volumes/"+seed.ID, map[string]string{"title": "Editor's Direct Edit"})

	rec := doPost(t, editor, fmt.Sprintf("/volumes/%s/versions/2/accept", seed.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp reviewVersionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Conflicts) != 1 || resp.Conflicts[0] != "title" {
		t.Errorf("Conflicts = %v, want [title]", resp.Conflicts)
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Editor's Direct Edit" {
		t.Errorf("Title = %q, want editor's direct edit to survive, not the stale submission value", got.Title)
	}
}

func TestSetCurrentVolumeVersionAdminOnly(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	editor := newTestRouter(t, []string{authz.RoleEditor})
	doPatch(t, editor, "/volumes/"+seed.ID, map[string]string{"title": "V2 Title"})

	forbidden := doPost(t, editor, fmt.Sprintf("/volumes/%s/versions/1/current", seed.ID), nil)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("editor rollback status = %d, want %d", forbidden.Code, http.StatusForbidden)
	}

	admin := newTestRouter(t, []string{authz.RoleAdmin})
	rec := doPost(t, admin, fmt.Sprintf("/volumes/%s/versions/1/current", seed.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin rollback status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Original Title" {
		t.Errorf("live Title = %q, want rollback to restore %q", got.Title, "Original Title")
	}
}
