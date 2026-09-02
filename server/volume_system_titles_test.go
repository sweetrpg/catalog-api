package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-data.go/gamesystems"
	"github.com/sweetrpg/catalog-objects.go/vo"
)

// fakeGameSystemsAPI returns a game system named `name` for /systems/<wantID>, and 404/500 for
// anything else per `status`.
func fakeGameSystemsAPI(t *testing.T, wantID, name string, otherStatus int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/systems/"+wantID {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"record_id": "meta-" + wantID, "name": name, "edition": "1e",
				"submitted_by": "seed", "submitted_at": "2026-01-01T00:00:00Z",
			})
			return
		}
		w.WriteHeader(otherStatus)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func withGameSystemsClient(t *testing.T, baseURL string) {
	t.Helper()
	prev := data.GameSystemsClient
	data.GameSystemsClient = gamesystems.NewClient(baseURL)
	t.Cleanup(func() { data.GameSystemsClient = prev })
}

func TestResolveSystemTitlesFromGameSystemsAPI(t *testing.T) {
	withGameSystemsClient(t, fakeGameSystemsAPI(t, "sysA", "Numenera", http.StatusNotFound))

	titles := resolveSystemTitles(context.Background(), []string{"sysA"}, nil)
	if titles["sysA"] != "Numenera" {
		t.Fatalf("titles = %v, want sysA -> Numenera", titles)
	}
}

func TestResolveSystemTitlesUnreachableFallsBackToHint(t *testing.T) {
	withGameSystemsClient(t, fakeGameSystemsAPI(t, "known", "Known", http.StatusInternalServerError))

	titles := resolveSystemTitles(context.Background(), []string{"sysA"}, map[string]string{"sysA": "Hinted"})
	if titles["sysA"] != "Hinted" {
		t.Errorf("with hint: got %v, want sysA -> Hinted", titles)
	}

	noHint := resolveSystemTitles(context.Background(), []string{"sysA"}, nil)
	if _, present := noHint["sysA"]; present {
		t.Errorf("no hint + unreachable: expected sysA left unset, got %v", noHint)
	}
}

func TestSyncSystemTitleUpdatesReferencingVolumes(t *testing.T) {
	ctx := context.Background()

	volID, err := data.AddVolume(ctx, &vo.VolumeVO{
		Title:        "Refs sysS",
		Systems:      []*vo.SystemVO{{ID: "sysS"}},
		SystemTitles: map[string]string{"sysS": "Old Name"},
	})
	if err != nil {
		t.Fatalf("seed volume: %v", err)
	}

	store := persistence.NewInMemoryStore(0)
	_ = store.Set(cache.CreateKey("/volumes/"+*volID), []byte("stale"), 0)

	if err := SyncSystemTitle(store)(ctx, "sysS", "New Name"); err != nil {
		t.Fatalf("SyncSystemTitle: %v", err)
	}

	v, err := data.GetVolume(ctx, *volID)
	if err != nil {
		t.Fatalf("get volume: %v", err)
	}
	if v.SystemTitles["sysS"] != "New Name" {
		t.Errorf("stored title = %q, want New Name", v.SystemTitles["sysS"])
	}
	var cached []byte
	if err := store.Get(cache.CreateKey("/volumes/"+*volID), &cached); err == nil {
		t.Errorf("expected /volumes/%s cache entry invalidated, still present", *volID)
	}
}

func TestSyncSystemTitleNoMatchStillSucceeds(t *testing.T) {
	store := persistence.NewInMemoryStore(0)
	if err := SyncSystemTitle(store)(context.Background(), "system-nothing-references", "X"); err != nil {
		t.Fatalf("no-match event should succeed, got %v", err)
	}
}
