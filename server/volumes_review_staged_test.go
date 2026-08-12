package server

import (
	"net/http"
	"testing"

	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-api/proposedchanges"
	"github.com/sweetrpg/catalog-data.go/data"
)

func TestAcceptPromotesStagedCover(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleSubmitter})

	seedStagedAsset(t, deps, "cover-staged", "user-3", []byte("cover-bytes"))
	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID:           seed.ID,
		Fields:             map[string]any{"title": "Proposed Title"},
		StagedCoverAssetId: "user-3",
	})
	finalizeRec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if finalizeRec.Code != http.StatusAccepted {
		t.Fatalf("finalize status = %d, want %d, body: %s", finalizeRec.Code, http.StatusAccepted, finalizeRec.Body.String())
	}

	editorDeps := newTestDepsWithAssets(t, []string{authz.RoleEditor}, deps.AssetsURL)
	pending, err := proposedchanges.ListPending(t.Context(), "volume", seed.ID)
	if err != nil || len(pending) == 0 {
		t.Fatalf("ListPending() = %v, %v", pending, err)
	}
	proposalID := pending[0].ID.Hex()

	acceptRec := doPost(t, editorDeps.Router, "/volumes/"+seed.ID+"/proposed-changes/"+proposalID+"/accept", nil)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept status = %d, want %d, body: %s", acceptRec.Code, http.StatusOK, acceptRec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.CoverAssetId != seed.ID {
		t.Errorf("CoverAssetId = %q, want %q (promoted)", got.CoverAssetId, seed.ID)
	}
}

func TestRejectReclaimsStagedCoverWithoutPromoting(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleSubmitter})

	seedStagedAsset(t, deps, "cover-staged", "user-4", []byte("cover-bytes"))
	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID:           seed.ID,
		Fields:             map[string]any{"title": "Proposed Title"},
		StagedCoverAssetId: "user-4",
	})
	finalizeRec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if finalizeRec.Code != http.StatusAccepted {
		t.Fatalf("finalize status = %d, want %d, body: %s", finalizeRec.Code, http.StatusAccepted, finalizeRec.Body.String())
	}

	editorDeps := newTestDepsWithAssets(t, []string{authz.RoleEditor}, deps.AssetsURL)
	pending, err := proposedchanges.ListPending(t.Context(), "volume", seed.ID)
	if err != nil || len(pending) == 0 {
		t.Fatalf("ListPending() = %v, %v", pending, err)
	}
	proposalID := pending[0].ID.Hex()

	rejectRec := doPost(t, editorDeps.Router, "/volumes/"+seed.ID+"/proposed-changes/"+proposalID+"/reject", nil)
	if rejectRec.Code != http.StatusOK {
		t.Fatalf("reject status = %d, want %d, body: %s", rejectRec.Code, http.StatusOK, rejectRec.Body.String())
	}

	got, err := data.GetVolume(t.Context(), seed.ID)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}
	if got.CoverAssetId != "" {
		t.Errorf("CoverAssetId = %q, want unset - a rejected proposal must not promote its staged cover", got.CoverAssetId)
	}
}
