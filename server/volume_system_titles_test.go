package server

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
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

func TestResolveSystemRefsStoresTheDocumentID(t *testing.T) {
	withGameSystemsClient(t, fakeGameSystemsAPI(t, "slug-a", "Numenera", http.StatusNotFound))

	// The fake answers /systems/slug-a with record_id meta-slug-a; the repeat must collapse.
	refs, titles := resolveSystemRefs(context.Background(), []*vo.SystemVO{{ID: "slug-a"}, {ID: "slug-a"}, {ID: "unknown"}}, nil)

	if got := systemIDsOf(refs); !slices.Equal(got, []string{"meta-slug-a", "unknown"}) {
		t.Errorf("refs = %v, want [meta-slug-a unknown]", got)
	}
	if !maps.Equal(titles, map[string]string{"meta-slug-a": "Numenera"}) {
		t.Errorf("titles = %v, want meta-slug-a -> Numenera", titles)
	}
}

func TestResolveSystemRefsUnreachableFallsBackToHint(t *testing.T) {
	withGameSystemsClient(t, fakeGameSystemsAPI(t, "known", "Known", http.StatusInternalServerError))
	requested := []*vo.SystemVO{{ID: "sysA"}}

	refs, titles := resolveSystemRefs(context.Background(), requested, map[string]string{"sysA": "Hinted"})
	if titles["sysA"] != "Hinted" || !slices.Equal(systemIDsOf(refs), []string{"sysA"}) {
		t.Errorf("with hint: refs %v titles %v, want sysA -> Hinted", systemIDsOf(refs), titles)
	}

	_, noHint := resolveSystemRefs(context.Background(), requested, nil)
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
