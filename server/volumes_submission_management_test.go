package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/editsession"
	"github.com/sweetrpg/catalog-api/submissioncap"
	"github.com/sweetrpg/catalog-data.go/data"
)

func TestFinalizeBlocksAtCapAndUnblocksAfterRetraction(t *testing.T) {
	// Other tests in this package share the same fake subject ("auth0|test-reviewer") and may
	// leave submitted versions of their own behind - the cap is per-user, not per-volume, so it
	// must be set relative to whatever's already pending, not a hardcoded number.
	baseline, err := data.CountSubmittedVolumeVersionsBySubmitter(t.Context(), "auth0|test-reviewer")
	if err != nil {
		t.Fatalf("CountSubmittedVolumeVersionsBySubmitter() error = %v", err)
	}
	t.Setenv(submissioncap.DefaultCapEnvVar, strconv.Itoa(int(baseline)+1))

	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleSubmitter})

	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID: seed.ID,
		Fields:   map[string]any{"title": "First Submission"},
	})
	first := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first finalize status = %d, want %d, body: %s", first.Code, http.StatusAccepted, first.Body.String())
	}
	var firstVersion submittedVersionResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstVersion); err != nil {
		t.Fatalf("unmarshal first finalize response: %v", err)
	}

	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID: seed.ID,
		Fields:   map[string]any{"title": "Second Submission"},
	})
	second := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if second.Code != http.StatusBadRequest {
		t.Fatalf("second finalize status = %d, want %d (cap reached), body: %s", second.Code, http.StatusBadRequest, second.Body.String())
	}

	// The session must survive a cap-rejected finalize so the user can retry.
	session, err := editsession.NewStore(deps.RedisPool).Get(t.Context(), "auth0|test-reviewer", "volume")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if session == nil {
		t.Fatal("session deleted after a cap-rejected finalize, want preserved")
	}

	retractPath := "/volumes/" + seed.ID + "/versions/" + strconv.Itoa(firstVersion.Version) + "/retract"
	retractRec := doPost(t, deps.Router, retractPath, nil)
	if retractRec.Code != http.StatusOK {
		t.Fatalf("retract status = %d, want %d, body: %s", retractRec.Code, http.StatusOK, retractRec.Body.String())
	}

	third := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if third.Code != http.StatusAccepted {
		t.Fatalf("finalize after retraction status = %d, want %d, body: %s", third.Code, http.StatusAccepted, third.Body.String())
	}
}

func TestAdminOverrideChangesOnlyOneUsersCap(t *testing.T) {
	t.Setenv(submissioncap.DefaultCapEnvVar, "25")
	admin := newTestDeps(t, []string{authz.RoleAdmin})

	req := doRequest(t, admin.Router, http.MethodPut, "/users/user-a/submission-cap", map[string]any{"cap": 1})
	if req.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d, body: %s", req.Code, http.StatusOK, req.Body.String())
	}

	got, err := submissioncap.CapFor(t.Context(), "user-a")
	if err != nil {
		t.Fatalf("CapFor() error = %v", err)
	}
	if got != 1 {
		t.Errorf("CapFor(user-a) = %d, want 1", got)
	}

	other, err := submissioncap.CapFor(t.Context(), "user-b")
	if err != nil {
		t.Fatalf("CapFor() error = %v", err)
	}
	if other != 25 {
		t.Errorf("CapFor(user-b) = %d, want 25 (unaffected)", other)
	}
}

func TestOnlyAdminCanSetSubmissionCap(t *testing.T) {
	editor := newTestDeps(t, []string{authz.RoleEditor})
	rec := doRequest(t, editor.Router, http.MethodPut, "/users/user-c/submission-cap", map[string]any{"cap": 1})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestRetractChangesStatusWithoutAffectingLiveRecord(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleSubmitter})
	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID: seed.ID,
		Fields:   map[string]any{"title": "Proposed Title"},
	})
	finalize := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if finalize.Code != http.StatusAccepted {
		t.Fatalf("finalize status = %d, want %d, body: %s", finalize.Code, http.StatusAccepted, finalize.Body.String())
	}
	var finalized submittedVersionResponse
	if err := json.Unmarshal(finalize.Body.Bytes(), &finalized); err != nil {
		t.Fatalf("unmarshal finalize response: %v", err)
	}

	retractPath := "/volumes/" + seed.ID + "/versions/" + strconv.Itoa(finalized.Version) + "/retract"
	rec := doPost(t, deps.Router, retractPath, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	version, err := data.GetVolumeVersion(t.Context(), seed.ID, finalized.Version)
	if err != nil {
		t.Fatalf("GetVolumeVersion() error = %v", err)
	}
	if string(version.State) != "withdrawn" {
		t.Errorf("State = %q, want %q", version.State, "withdrawn")
	}
}

func TestPullBackCreatesSessionAndRetractsSource(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDeps(t, []string{authz.RoleSubmitter})
	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID: seed.ID,
		Fields:   map[string]any{"title": "Proposed Title"},
	})
	finalize := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if finalize.Code != http.StatusAccepted {
		t.Fatalf("finalize status = %d, want %d, body: %s", finalize.Code, http.StatusAccepted, finalize.Body.String())
	}
	var finalized submittedVersionResponse
	if err := json.Unmarshal(finalize.Body.Bytes(), &finalized); err != nil {
		t.Fatalf("unmarshal finalize response: %v", err)
	}

	pullBackPath := "/volumes/" + seed.ID + "/versions/" + strconv.Itoa(finalized.Version) + "/pull-back"
	rec := doPost(t, deps.Router, pullBackPath, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	version, err := data.GetVolumeVersion(t.Context(), seed.ID, finalized.Version)
	if err != nil {
		t.Fatalf("GetVolumeVersion() error = %v", err)
	}
	if string(version.State) != "withdrawn" {
		t.Errorf("State = %q, want %q", version.State, "withdrawn")
	}

	session, err := editsession.NewStore(deps.RedisPool).Get(t.Context(), "auth0|test-reviewer", "volume")
	if err != nil {
		t.Fatalf("Get() session error = %v", err)
	}
	if session == nil {
		t.Fatal("no session created by pull-back")
	}
	if session.RecordID != seed.ID {
		t.Errorf("session.RecordID = %q, want %q", session.RecordID, seed.ID)
	}
	if session.Fields["title"] != "Proposed Title" {
		t.Errorf("session.Fields[title] = %v, want %q", session.Fields["title"], "Proposed Title")
	}
}

func TestPullBackConflictsWithExistingSession(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	other := seedVolume(t, "Other Volume")
	deps := newTestDeps(t, []string{authz.RoleSubmitter})
	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID: seed.ID,
		Fields:   map[string]any{"title": "Proposed Title"},
	})
	finalize := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if finalize.Code != http.StatusAccepted {
		t.Fatalf("finalize status = %d, want %d, body: %s", finalize.Code, http.StatusAccepted, finalize.Body.String())
	}
	var finalized submittedVersionResponse
	if err := json.Unmarshal(finalize.Body.Bytes(), &finalized); err != nil {
		t.Fatalf("unmarshal finalize response: %v", err)
	}

	// A different in-flight session (for a different volume) is already open.
	seedEditSession(t, deps, "auth0|test-reviewer", "volume", editsession.Session{
		RecordID: other.ID,
		Fields:   map[string]any{"title": "Unrelated in-flight edit"},
	})

	pullBackPath := "/volumes/" + seed.ID + "/versions/" + strconv.Itoa(finalized.Version) + "/pull-back"
	rec := doPost(t, deps.Router, pullBackPath, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	// The version must be untouched - "creates the session and retracts the source, or does
	// neither".
	version, err := data.GetVolumeVersion(t.Context(), seed.ID, finalized.Version)
	if err != nil {
		t.Fatalf("GetVolumeVersion() error = %v", err)
	}
	if string(version.State) != "submitted" {
		t.Errorf("State = %q, want still %q after a conflicting pull-back", version.State, "submitted")
	}
}
