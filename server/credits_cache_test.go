package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/sweetrpg/authz-client.go/authz"
)

// An editor finalize / live PATCH that changes credits mutates the contributions collection
// directly (they are not on the versioned volume), so the /contributions list cache and each
// affected person's cache must be busted - otherwise the Credits panel serves a stale list.
func TestApplyVolumePatchCreditsBustsContributionCaches(t *testing.T) {
	deps := newTestDeps(t, []string{authz.RoleEditor})
	vol := seedVolume(t, "Credits Cache Volume")
	personID := seedPerson(t, "Ada Contributor")

	prime := func(path string) {
		if err := deps.Cache.Set(cache.CreateKey(path), []byte("stale"), time.Hour); err != nil {
			t.Fatalf("prime cache %s: %v", path, err)
		}
	}
	missing := func(path string) bool {
		var v []byte
		return deps.Cache.Get(cache.CreateKey(path), &v) == persistence.ErrCacheMiss
	}

	prime("/contributions")
	prime("/volumes/" + vol.ID)
	prime("/persons/" + personID)
	prime("/persons/" + personID + "/volumes")

	rec := doPatch(t, deps.Router, "/volumes/"+vol.ID, map[string]any{
		"credits": []map[string]string{{"personId": personID, "contributionType": "author"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /volumes: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	for _, path := range []string{
		"/contributions",
		"/volumes/" + vol.ID,
		"/persons/" + personID,
		"/persons/" + personID + "/volumes",
	} {
		if !missing(path) {
			t.Errorf("cache for %s was not busted after a credits change", path)
		}
	}
}

// A no-credits PATCH must not touch the contributions cache.
func TestApplyVolumePatchWithoutCreditsLeavesContributionCache(t *testing.T) {
	deps := newTestDeps(t, []string{authz.RoleEditor})
	vol := seedVolume(t, "No Credits Volume")

	if err := deps.Cache.Set(cache.CreateKey("/contributions"), []byte("stale"), time.Hour); err != nil {
		t.Fatalf("prime cache: %v", err)
	}

	rec := doPatch(t, deps.Router, "/volumes/"+vol.ID, map[string]any{"title": "Retitled"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /volumes: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	var v []byte
	if err := deps.Cache.Get(cache.CreateKey("/contributions"), &v); err == persistence.ErrCacheMiss {
		t.Error("/contributions cache was busted by a PATCH that did not change credits")
	}
}
