package server

import (
	"bytes"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/jsonapi"
	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/catalog-data.go/data"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func countManyPayload(t *testing.T, body []byte, proto any) int {
	t.Helper()
	raw, err := jsonapi.UnmarshalManyPayload(bytes.NewReader(body), reflect.TypeOf(proto))
	if err != nil {
		t.Fatalf("unmarshal payload: %v (body: %s)", err, body)
	}
	return len(raw)
}

func TestGetVolumeContributionsReturnsOnlyThatVolumes(t *testing.T) {
	target := seedVolume(t, "Target Volume")
	other := seedVolume(t, "Other Volume")
	personA := seedPerson(t, "Person A")
	personB := seedPerson(t, "Person B")

	if _, err := data.AddContribution(t.Context(), personA, target.ID, "author", "test"); err != nil {
		t.Fatalf("seed target contribution: %v", err)
	}
	if _, err := data.AddContribution(t.Context(), personB, target.ID, "editor", "test"); err != nil {
		t.Fatalf("seed second target contribution: %v", err)
	}
	if _, err := data.AddContribution(t.Context(), personB, other.ID, "author", "test"); err != nil {
		t.Fatalf("seed unrelated contribution: %v", err)
	}

	r := newTestRouter(t, []string{authz.RoleEditor})
	rec := doGet(t, r, "/volumes/"+target.ID+"/contributions")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if n := countManyPayload(t, rec.Body.Bytes(), new(vo.ContributionVO)); n != 2 {
		t.Fatalf("contribution count = %d, want 2", n)
	}
}

func TestGetVolumeContributionsEmptyWhenNone(t *testing.T) {
	target := seedVolume(t, "Creditless Volume")

	r := newTestRouter(t, []string{authz.RoleEditor})
	rec := doGet(t, r, "/volumes/"+target.ID+"/contributions")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if n := countManyPayload(t, rec.Body.Bytes(), new(vo.ContributionVO)); n != 0 {
		t.Fatalf("contribution count = %d, want 0", n)
	}
}

func seedReview(t *testing.T, volumeID, title string) {
	t.Helper()
	if _, err := database.Insert("reviews", catalogmodels.Review{
		ID:       primitive.NewObjectID().Hex(),
		Title:    title,
		Body:     "seeded",
		VolumeId: volumeID,
	}); err != nil {
		t.Fatalf("seed review: %v", err)
	}
}

func TestGetVolumeReviewsReturnsOnlyThatVolumes(t *testing.T) {
	target := seedVolume(t, "Reviewed Volume")
	other := seedVolume(t, "Other Reviewed Volume")
	seedReview(t, target.ID, "Target Review 1")
	seedReview(t, target.ID, "Target Review 2")
	seedReview(t, other.ID, "Unrelated Review")

	r := newTestRouter(t, []string{authz.RoleEditor})
	rec := doGet(t, r, "/volumes/"+target.ID+"/reviews")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if n := countManyPayload(t, rec.Body.Bytes(), new(vo.ReviewVO)); n != 2 {
		t.Fatalf("review count = %d, want 2", n)
	}
}

func TestGetVolumeReviewsEmptyWhenNone(t *testing.T) {
	target := seedVolume(t, "Unreviewed Volume")

	r := newTestRouter(t, []string{authz.RoleEditor})
	rec := doGet(t, r, "/volumes/"+target.ID+"/reviews")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if n := countManyPayload(t, rec.Body.Bytes(), new(vo.ReviewVO)); n != 0 {
		t.Fatalf("review count = %d, want 0", n)
	}
}
