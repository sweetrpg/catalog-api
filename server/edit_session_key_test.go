package server

import (
	"net/http"
	"testing"

	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/catalog-api/editsession"
)

// The shared edit-session store is keyed by the raw IdP subject (catalog-web, the writer, has
// no canonical users._id). catalog-api must read/write it by authz.Subject, not authz.Viewer -
// keying by the canonical id was the "400 no_session on finalize" dev bug.
const tokenSub = "auth0|test-reviewer"
const canonicalID = "b3384f5d-78c1-4965-8112-37395c2b8ef3"

func TestFinalizeSessionKeyedByTokenSubject(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDepsDistinctProfileID(t, []string{authz.RoleEditor}, canonicalID)

	// Session written under the token subject, exactly as catalog-web keys it.
	seedEditSession(t, deps, tokenSub, "volume", editsession.Session{
		RecordID: seed.ID,
		Fields:   map[string]any{"title": "Finalized Title"},
	})

	rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d - handler must find the session keyed by token sub, body: %s",
			rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestFinalizeSessionIgnoresSessionKeyedByCanonicalID(t *testing.T) {
	seed := seedVolume(t, "Original Title")
	deps := newTestDepsDistinctProfileID(t, []string{authz.RoleEditor}, canonicalID)

	// A session mistakenly keyed by the canonical users._id must not be picked up.
	seedEditSession(t, deps, canonicalID, "volume", editsession.Session{
		RecordID: seed.ID,
		Fields:   map[string]any{"title": "Wrong Key"},
	})

	rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/finalize-session", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (no_session) - a canonical-id-keyed session must not match, body: %s",
			rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPullBackVolumeVersionSessionKeyedByTokenSubject(t *testing.T) {
	seed := seedVolume(t, "Pullback Base")
	deps := newTestDepsDistinctProfileID(t, []string{authz.RoleSubmitter}, canonicalID)

	// Create a submitted version to pull back.
	patch := map[string]any{"title": "Submitted Edit"}
	if rec := doPatch(t, deps.Router, "/volumes/"+seed.ID, patch); rec.Code != http.StatusAccepted {
		t.Fatalf("seed submitted version: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	rec := doPost(t, deps.Router, "/volumes/"+seed.ID+"/versions/2/pull-back", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("pull-back status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}

	// The session pull-back created must be readable under the token subject key.
	session, err := editsession.NewStore(deps.RedisPool).Get(t.Context(), tokenSub, "volume")
	if err != nil {
		t.Fatalf("Get(tokenSub): %v", err)
	}
	if session == nil {
		t.Fatal("pull-back session not found under token-subject key")
	}
	if got, err := editsession.NewStore(deps.RedisPool).Get(t.Context(), canonicalID, "volume"); err != nil || got != nil {
		t.Fatalf("pull-back session leaked under canonical-id key: got=%v err=%v", got, err)
	}
}
