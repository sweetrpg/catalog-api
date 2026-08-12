package server

import (
	"net/http"
	"testing"

	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-data.go/data"
)

func TestFinalizeSessionNoSession404s(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleEditor})

	rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestFinalizeSessionEditorAppliesFieldsAndPromotesCover(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleEditor})

	seedStagedAsset(t, deps, "cover-staged", "user-1", []byte("cover-bytes"))
	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID:           seed.ID,
		Fields:             map[string]any{"title": "Finalized Title"},
		StagedCoverAssetId: "user-1",
	})

	rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Finalized Title" {
		t.Errorf("Title = %q, want %q", got.Title, "Finalized Title")
	}
	if got.CoverAssetId != seed.ID {
		t.Errorf("CoverAssetId = %q, want %q", got.CoverAssetId, seed.ID)
	}

	session, err := editsession.NewStore(deps.RedisPool).Get(t.Context(), "auth0|test-reviewer", "volume")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if session != nil {
		t.Error("session still exists after successful finalize, want deleted")
	}
}

func TestFinalizeSessionSubmitterCreatesProposalWithStagedCover(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleSubmitter})

	seedStagedAsset(t, deps, "cover-staged", "user-2", []byte("cover-bytes"))
	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID:           seed.ID,
		Fields:             map[string]any{"title": "Proposed Title"},
		StagedCoverAssetId: "user-2",
	})

	rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.Title != "Original Title" || got.CoverAssetId != "" {
		t.Errorf("live record changed by submitter finalize: Title=%q CoverAssetId=%q", got.Title, got.CoverAssetId)
	}
}

func TestFinalizeSessionRecordMismatch400s(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	other := seedVolume(t, "Other Volume")
	deps := newTestDeps(t, []string{authz.RoleEditor})

	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID: other.ID,
		Fields:   map[string]any{"title": "Should Not Apply"},
	})

	rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
