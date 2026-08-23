package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-objects.go/vo"
)

// newPublisherTestRouter/newPersonTestRouter mirror newStudioTestRouter
// (entity_version_patch_test.go) - only the GET .../volumes routes are exercised here, so the
// role granted to the fake auth-api doesn't matter for these tests.
func newPublisherTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return newRelationshipTestRouter(t, setupPublisherHandlers)
}

func newPersonTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return newRelationshipTestRouter(t, setupPersonHandlers)
}

func newRelationshipTestRouter(t *testing.T, setup func(*gin.Engine, persistence.CacheStore, cachettl.Config, *authz.Client)) *gin.Engine {
	t.Helper()

	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleEditor}, Sub: "auth0|test-reviewer"})
	}))
	t.Cleanup(authAPI.Close)

	r := gin.New()
	setup(r, persistence.NewInMemoryStore(0), cachettl.Config{}, authz.NewClient(authAPI.URL))
	return r
}

// volumeTitles unmarshals a JSON:API volume list response and returns each volume's title, in
// response order, for assertion against seeded fixtures.
func volumeTitles(t *testing.T, body []byte) []string {
	t.Helper()

	raw, err := jsonapi.UnmarshalManyPayload(bytes.NewReader(body), reflect.TypeOf(new(vo.VolumeVO)))
	if err != nil {
		t.Fatalf("unmarshal volumes payload: %v (body: %s)", err, body)
	}
	titles := make([]string, 0, len(raw))
	for _, r := range raw {
		v, ok := r.(*vo.VolumeVO)
		if !ok {
			t.Fatalf("unexpected element type %T in volumes payload", r)
		}
		titles = append(titles, v.Title)
	}
	return titles
}

func TestGetPublisherVolumesReturnsOnlyAssociatedVolumes(t *testing.T) {
	publisherID := seedPublisher(t, "Assoc Publisher")
	otherID := seedPublisher(t, "Other Publisher")
	if _, err := data.AddVolume(t.Context(), &vo.VolumeVO{Title: "Linked Volume", Publishers: []*vo.PublisherVO{{ID: publisherID}}}); err != nil {
		t.Fatalf("seed linked volume: %v", err)
	}
	if _, err := data.AddVolume(t.Context(), &vo.VolumeVO{Title: "Unlinked Volume", Publishers: []*vo.PublisherVO{{ID: otherID}}}); err != nil {
		t.Fatalf("seed unlinked volume: %v", err)
	}

	r := newPublisherTestRouter(t)
	rec := doGet(t, r, "/publishers/"+publisherID+"/volumes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	titles := volumeTitles(t, rec.Body.Bytes())
	if want := []string{"Linked Volume"}; !reflect.DeepEqual(titles, want) {
		t.Fatalf("titles = %v, want %v", titles, want)
	}
}

func TestGetPublisherVolumesEmptyWhenNoneAssociated(t *testing.T) {
	publisherID := seedPublisher(t, "Lonely Publisher")

	r := newPublisherTestRouter(t)
	rec := doGet(t, r, "/publishers/"+publisherID+"/volumes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if titles := volumeTitles(t, rec.Body.Bytes()); len(titles) != 0 {
		t.Fatalf("titles = %v, want none", titles)
	}
}

func TestGetStudioVolumesReturnsOnlyAssociatedVolumes(t *testing.T) {
	studioID := seedStudio(t, "Assoc Studio")
	otherID := seedStudio(t, "Other Studio")
	if _, err := data.AddVolume(t.Context(), &vo.VolumeVO{Title: "Linked Volume", Studios: []*vo.StudioVO{{ID: studioID}}}); err != nil {
		t.Fatalf("seed linked volume: %v", err)
	}
	if _, err := data.AddVolume(t.Context(), &vo.VolumeVO{Title: "Unlinked Volume", Studios: []*vo.StudioVO{{ID: otherID}}}); err != nil {
		t.Fatalf("seed unlinked volume: %v", err)
	}

	r := newRelationshipTestRouter(t, setupStudioHandlers)
	rec := doGet(t, r, "/studios/"+studioID+"/volumes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	titles := volumeTitles(t, rec.Body.Bytes())
	if want := []string{"Linked Volume"}; !reflect.DeepEqual(titles, want) {
		t.Fatalf("titles = %v, want %v", titles, want)
	}
}

func TestGetStudioVolumesEmptyWhenNoneAssociated(t *testing.T) {
	studioID := seedStudio(t, "Lonely Studio")

	r := newRelationshipTestRouter(t, setupStudioHandlers)
	rec := doGet(t, r, "/studios/"+studioID+"/volumes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if titles := volumeTitles(t, rec.Body.Bytes()); len(titles) != 0 {
		t.Fatalf("titles = %v, want none", titles)
	}
}

func TestGetPersonVolumesReturnsOnlyContributedVolumes(t *testing.T) {
	personID := seedPerson(t, "Assoc Person")
	otherPersonID := seedPerson(t, "Other Person")

	linked := seedVolume(t, "Linked Volume")
	unlinked := seedVolume(t, "Unlinked Volume")
	if _, err := data.AddContribution(t.Context(), personID, linked.ID, "author", "test"); err != nil {
		t.Fatalf("seed contribution: %v", err)
	}
	if _, err := data.AddContribution(t.Context(), otherPersonID, unlinked.ID, "author", "test"); err != nil {
		t.Fatalf("seed unrelated contribution: %v", err)
	}

	r := newPersonTestRouter(t)
	rec := doGet(t, r, "/persons/"+personID+"/volumes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	titles := volumeTitles(t, rec.Body.Bytes())
	if want := []string{"Linked Volume"}; !reflect.DeepEqual(titles, want) {
		t.Fatalf("titles = %v, want %v", titles, want)
	}
}

func TestGetPersonVolumesEmptyWhenNoneContributed(t *testing.T) {
	personID := seedPerson(t, "Lonely Person")

	r := newPersonTestRouter(t)
	rec := doGet(t, r, "/persons/"+personID+"/volumes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if titles := volumeTitles(t, rec.Body.Bytes()); len(titles) != 0 {
		t.Fatalf("titles = %v, want none", titles)
	}
}
