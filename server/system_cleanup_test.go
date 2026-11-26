package server

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/catalog-api/internal/events"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCleanupDeletedSystemRemovesAnnouncesAndFlushesOnce(t *testing.T) {
	ctx := context.Background()
	pub, next := captureEvents(t)
	volumeEventPublisher = pub
	deletion := events.SystemDeletion{ID: primitive.NewObjectID().Hex()}
	id, err := data.AddVolume(ctx, &vo.VolumeVO{Title: "Cleaned", Systems: []*vo.SystemVO{{ID: deletion.ID}}})
	if err != nil {
		t.Fatalf("seed volume: %v", err)
	}
	flushes := 0
	cleanup := CleanupDeletedSystem(func() { flushes++ })

	if err := cleanup(ctx, deletion); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	assertEvent(t, next("catalog.events.volume.updated"), *id, 2)
	volume, err := data.GetVolume(ctx, *id)
	if err != nil || len(volume.Systems) != 0 {
		t.Fatalf("volume systems = %v, err %v", volume.Systems, err)
	}
	if err := cleanup(ctx, deletion); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if flushes != 1 {
		t.Errorf("flushes = %d, want 1: redelivery changes nothing", flushes)
	}
	if versions, err := data.ListVolumeVersions(ctx, *id); err != nil || len(versions) != 2 {
		t.Errorf("versions = %d, err %v, want 2", len(versions), err)
	}

	recorded, err := RecordedSystemDeletions(ctx)
	if err != nil || !slices.ContainsFunc(recorded, func(d events.SystemDeletion) bool { return d.ID == deletion.ID }) {
		t.Errorf("deletion not recorded: err %v", err)
	}
}

func TestCreateVolumeStoresDocumentIDAndTitle(t *testing.T) {
	withGameSystemsClient(t, fakeGameSystemsAPI(t, "numenera", "Numenera", http.StatusNotFound))
	router := newTestRouter(t, []string{authz.RoleAdmin})

	rec := doPost(t, router, "/volumes", map[string]any{"title": "By slug", "systemIds": []string{"numenera"}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}

	volume, err := data.GetVolume(t.Context(), decodeVolume(t, rec.Body.Bytes()).ID)
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if got := systemIDsOf(volume.Systems); !slices.Equal(got, []string{"meta-numenera"}) {
		t.Errorf("systems = %v, want the document ID", got)
	}
	if volume.SystemTitles["meta-numenera"] != "Numenera" {
		t.Errorf("titles = %v", volume.SystemTitles)
	}
}
