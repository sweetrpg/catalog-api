package server

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-data.go/data"
)

const (
	testSubject = "auth0|test-reviewer"
	testOwner   = "auth0-test-reviewer"
)

// failingAssets proxies to a fake asset store, answering 500 to any request fail matches.
func failingAssets(t *testing.T, upstream string, fail func(*http.Request) bool) string {
	t.Helper()
	target, err := url.Parse(upstream)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail(r) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func assetExists(t *testing.T, assetsURL, kind, id string) bool {
	t.Helper()
	_, _, err := assets.NewClient(assetsURL).Get(t.Context(), "", kind, id)
	return err == nil
}

// submitStaged finalizes a submitter session holding the given staged files.
func submitStaged(t *testing.T, volumeID, cover string, samples []string) (testDeps, string) {
	t.Helper()
	deps := newTestDeps(t, []string{authz.RoleSubmitter})
	if cover != "" {
		seedStagedAsset(t, deps, "cover-staged", cover, []byte("cover-bytes"))
	}
	seedEditSession(t, deps, testSubject, "volume", editsession.Session{
		RecordID:           volumeID,
		Fields:             map[string]any{"title": "Proposed Title"},
		StagedCoverAssetId: cover,
		SampleAssetIds:     samples,
	})
	rec := doPost(t, deps.Router, "/volumes/"+volumeID+"/finalize-session", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("finalize status = %d, body: %s", rec.Code, rec.Body.String())
	}
	return deps, "/volumes/" + volumeID + "/versions/2"
}

func TestFinalizeRefusesUnexpectedStagedIDs(t *testing.T) {
	cases := map[string]editsession.Session{
		"other owner cover":    {StagedCoverAssetId: "auth0-other-owner"},
		"numbered cover":       {StagedCoverAssetId: testOwner + "-1"},
		"malformed cover":      {StagedCoverAssetId: testOwner + "/x"},
		"other owner sample":   {SampleAssetIds: []string{testOwner + "-0", "auth0-other-owner-0"}},
		"sample without index": {SampleAssetIds: []string{testOwner}},
		"sample bad index":     {SampleAssetIds: []string{testOwner + "-x"}},
		"sample empty index":   {SampleAssetIds: []string{testOwner + "-"}},
	}
	for _, role := range []string{authz.RoleSubmitter, authz.RoleEditor} {
		for name, session := range cases {
			t.Run(role+"/"+name, func(t *testing.T) {
				seed := seedVolume(t, "Original Title")
				deps := newTestDeps(t, []string{role})
				session.RecordID = seed.ID
				session.Fields = map[string]any{"title": "Proposed Title"}
				seedEditSession(t, deps, testSubject, "volume", session)

				rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)

				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_staged_asset") {
					t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
				}
				versions, err := data.ListVolumeVersions(t.Context(), seed.ID, internalReader)
				if err != nil || len(versions) != 1 {
					t.Fatalf("versions = %d, err = %v, want only the seed version", len(versions), err)
				}
			})
		}
	}
}

func TestAcceptKeepsStagedFilesWhenACopyFails(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	// The second sample was never uploaded, so its copy fails after the cover's succeeded.
	deps, version := submitStaged(t, seed.ID, testOwner, []string{testOwner + "-0"})
	editor := newTestDepsWithAssets(t, []string{authz.RoleEditor}, deps.AssetsURL)

	rec := doPost(t, editor.Router, version+"/accept", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if !assetExists(t, deps.AssetsURL, "cover-staged", testOwner) {
		t.Error("staged cover deleted by a failed accept")
	}
	got, err := data.GetVolumeVersion(t.Context(), seed.ID, 2, internalReader)
	if err != nil || got.State != "submitted" {
		t.Fatalf("version = %+v, err = %v, want still submitted", got, err)
	}

	// Once the missing file is there, the same accept succeeds and cleans up.
	seedStagedAsset(t, deps, "sample-staged", testOwner+"-0", []byte("sample-bytes"))
	if retry := doPost(t, editor.Router, version+"/accept", nil); retry.Code != http.StatusOK {
		t.Fatalf("retry status = %d, body: %s", retry.Code, retry.Body.String())
	}
	if assetExists(t, deps.AssetsURL, "cover-staged", testOwner) ||
		assetExists(t, deps.AssetsURL, "sample-staged", testOwner+"-0") {
		t.Error("staged files left after a successful accept")
	}
}

func TestAcceptDeletesStagedFilesOnlyAfterPublishing(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps, version := submitStaged(t, seed.ID, testOwner, nil)
	var statesAtDelete []string
	assetsURL := failingAssets(t, deps.AssetsURL, func(r *http.Request) bool {
		if r.Method == http.MethodDelete {
			got, err := data.GetVolumeVersion(r.Context(), seed.ID, 2, internalReader)
			if err == nil {
				statesAtDelete = append(statesAtDelete, string(got.State))
			}
		}
		return false
	})
	editor := newTestDepsWithAssets(t, []string{authz.RoleEditor}, assetsURL)

	rec := doPost(t, editor.Router, version+"/accept", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if len(statesAtDelete) != 1 || statesAtDelete[0] != "live" {
		t.Errorf("version states at delete = %v, want [live]", statesAtDelete)
	}
}

func TestAcceptSucceedsWhenStagedDeleteFails(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps, version := submitStaged(t, seed.ID, testOwner, nil)
	assetsURL := failingAssets(t, deps.AssetsURL, func(r *http.Request) bool { return r.Method == http.MethodDelete })
	editor := newTestDepsWithAssets(t, []string{authz.RoleEditor}, assetsURL)

	rec := doPost(t, editor.Router, version+"/accept", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil || got.CoverAssetId != seed.ID {
		t.Fatalf("volume = %+v, err = %v, want published cover", got, err)
	}
}

func TestAcceptOfReviewedVersionTouchesNoAssets(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps, version := submitStaged(t, seed.ID, testOwner, nil)
	editor := newTestDepsWithAssets(t, []string{authz.RoleEditor}, deps.AssetsURL)
	if rec := doPost(t, editor.Router, version+"/reject", nil); rec.Code != http.StatusOK {
		t.Fatalf("reject status = %d, body: %s", rec.Code, rec.Body.String())
	}

	rec := doPost(t, editor.Router, version+"/accept", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if assetExists(t, deps.AssetsURL, "cover", seed.ID) {
		t.Error("live cover written for a rejected version")
	}
	if !assetExists(t, deps.AssetsURL, "cover-staged", testOwner) {
		t.Error("staged cover deleted for a rejected version")
	}
}

func TestEditorFinalizeKeepsStagedFilesWhenACopyFails(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleEditor})
	seedStagedAsset(t, deps, "cover-staged", testOwner, []byte("cover-bytes"))
	seedEditSession(t, deps, testSubject, "volume", editsession.Session{
		RecordID:           seed.ID,
		Fields:             map[string]any{},
		StagedCoverAssetId: testOwner,
		SampleAssetIds:     []string{testOwner + "-0"},
	})

	rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if !assetExists(t, deps.AssetsURL, "cover-staged", testOwner) {
		t.Error("staged cover deleted by a failed finalize")
	}
}

func TestAcceptPublishesStagedSamples(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleSubmitter})
	samples := []string{testOwner + "-0", testOwner + "-1"}
	for i, id := range samples {
		seedStagedAsset(t, deps, "sample-staged", id, []byte("sample-"+strconv.Itoa(i)))
	}
	seedEditSession(t, deps, testSubject, "volume", editsession.Session{
		RecordID: seed.ID, Fields: map[string]any{}, SampleAssetIds: samples,
	})
	if rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("finalize status = %d, body: %s", rec.Code, rec.Body.String())
	}
	editor := newTestDepsWithAssets(t, []string{authz.RoleEditor}, deps.AssetsURL)

	rec := doPost(t, editor.Router, "/volumes/"+seed.ID+"/versions/2/accept", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
	}
	for i := range samples {
		if !assetExists(t, deps.AssetsURL, "sample", seed.ID+"-"+strconv.Itoa(i)) {
			t.Errorf("live sample %d missing", i)
		}
	}
}

// assets-web allows any decimal index, so a client can use a timestamp to keep names unique.
func TestFinalizeAcceptsLargeSampleIndex(t *testing.T) {
	seed := seedVolume(t, "Original Title")

	submitStaged(t, seed.ID, "", []string{testOwner + "-1795756800000"})
}
