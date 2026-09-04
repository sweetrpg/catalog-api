package server

import (
	"bytes"
	"net/http"
	"slices"
	"testing"

	"github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-objects.go/vo"
)

func decodeVolume(t *testing.T, body []byte) *vo.VolumeVO {
	t.Helper()
	var v vo.VolumeVO
	if err := jsonapi.UnmarshalPayload(bytes.NewReader(body), &v); err != nil {
		t.Fatalf("decode volume response: %v (body: %s)", err, body)
	}
	return &v
}

func TestCreateVolumeAdminReturnsLiveRecord(t *testing.T) {
	admin := newTestRouter(t, []string{authz.RoleAdmin})

	rec := doPost(t, admin, "/volumes", map[string]any{
		"title":       "Dungeon World",
		"description": "A fantasy world of adventure.",
		"tags":        []string{"Fantasy", "PbtA"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}
	created := decodeVolume(t, rec.Body.Bytes())
	if created.ID == "" || created.Title != "Dungeon World" {
		t.Fatalf("unexpected created volume: %+v", created)
	}

	got, err := data.GetVolume(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if got == nil {
		t.Fatal("created volume not found as a live record")
	}
	if got.Description != "A fantasy world of adventure." || len(got.Tags) != 2 {
		t.Errorf("live record did not persist fields: %+v", got)
	}
}

func TestCreateVolumeRejectsMissingTitle(t *testing.T) {
	admin := newTestRouter(t, []string{authz.RoleAdmin})

	for _, body := range []map[string]any{
		{"description": "no title"},
		{"title": "   "},
	} {
		rec := doPost(t, admin, "/volumes", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %v: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestCreateVolumeLinksPublisherOnCreate(t *testing.T) {
	admin := newTestRouter(t, []string{authz.RoleAdmin})
	publisherID := seedPublisher(t, "Evil Hat Productions")

	rec := doPost(t, admin, "/volumes", map[string]any{
		"title":        "Fate Core",
		"publisherIds": []string{publisherID},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}
	created := decodeVolume(t, rec.Body.Bytes())

	got, err := data.GetVolume(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	ids := make([]string, 0, len(got.Publishers))
	for _, p := range got.Publishers {
		if p != nil {
			ids = append(ids, p.ID)
		}
	}
	if !slices.Contains(ids, publisherID) {
		t.Errorf("created volume publishers = %v, want to contain %s", ids, publisherID)
	}
}

func TestCreateVolumeRequiresWriteRole(t *testing.T) {
	reader := newTestRouter(t, []string{})

	rec := doPost(t, reader, "/volumes", map[string]any{"title": "Should Not Create"})
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 or 403", rec.Code)
	}
}
