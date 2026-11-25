package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/internal/events"
)

// captureEvents starts an in-process JetStream and returns a publisher plus a function that
// waits for the next event on a subject.
func captureEvents(t *testing.T) (*events.Publisher, func(subject string) events.Envelope) {
	t.Helper()
	s, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir()})
	if err != nil {
		t.Fatalf("new NATS server: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	t.Cleanup(s.Shutdown)

	conn, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(conn.Close)
	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	stream, err := js.CreateStream(context.Background(), jetstream.StreamConfig{Name: "CATALOG_EVENTS", Subjects: []string{"catalog.events.>"}})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}

	t.Setenv("NATS_URL", s.ClientURL())
	t.Setenv("NATS_CREDS", "")
	pub, err := events.NewPublisher(context.Background())
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	t.Cleanup(pub.Close)
	previous := volumeEventPublisher
	t.Cleanup(func() { volumeEventPublisher = previous })

	return pub, func(subject string) events.Envelope {
		t.Helper()
		msg, err := stream.GetLastMsgForSubject(context.Background(), subject)
		if err != nil {
			t.Fatalf("no event on %s: %v", subject, err)
		}
		if err := stream.Purge(context.Background(), jetstream.WithPurgeSubject(subject)); err != nil {
			t.Fatalf("purge: %v", err)
		}
		var e events.Envelope
		if err := json.Unmarshal(msg.Data, &e); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		return e
	}
}

func assertEvent(t *testing.T, e events.Envelope, entityID string, revision int) {
	t.Helper()
	if e.EntityID != entityID || e.Revision != revision {
		t.Errorf("event %s.%s = id %s revision %d, want id %s revision %d", e.EntityType, e.Action, e.EntityID, e.Revision, entityID, revision)
	}
}

func TestVolumeMutationsPublishEvents(t *testing.T) {
	router := newTestRouter(t, []string{authz.RoleAdmin})
	pub, next := captureEvents(t)
	volumeEventPublisher = pub

	rec := doPost(t, router, "/volumes", map[string]any{"title": "Evented"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}
	id := decodeVolume(t, rec.Body.Bytes()).ID
	assertEvent(t, next("catalog.events.volume.created"), id, 1)

	doRequest(t, router, http.MethodDelete, "/volumes/"+id, nil)
	assertEvent(t, next("catalog.events.volume.deleted"), id, 0)

	doPost(t, router, "/volumes/"+id+"/restore", nil)
	assertEvent(t, next("catalog.events.volume.updated"), id, 1)

	doPost(t, router, "/volumes/"+id+"/versions/1/current", nil)
	assertEvent(t, next("catalog.events.volume.updated"), id, 1)
}

func TestCreditChangesPublishContributionEvents(t *testing.T) {
	router := newTestRouter(t, []string{authz.RoleAdmin})
	pub, next := captureEvents(t)
	volumeEventPublisher = pub
	volume := seedVolume(t, "Credited")
	personID := seedPerson(t, "Author")

	doPatch(t, router, "/volumes/"+volume.ID, map[string]any{
		"credits": []map[string]string{{"personId": personID, "contributionType": "author"}},
	})
	created := next("catalog.events.contribution.created")
	if created.EntityID == "" || created.Revision != 1 {
		t.Errorf("unexpected created event: %+v", created)
	}

	doPatch(t, router, "/volumes/"+volume.ID, map[string]any{"credits": []map[string]string{}})
	assertEvent(t, next("catalog.events.contribution.deleted"), created.EntityID, 0)
}

func TestRelationshipPatchPublishesVolumeUpdated(t *testing.T) {
	pub, next := captureEvents(t)
	router := newStudioTestRouter(t, []string{authz.RoleAdmin})
	volumeEventPublisher = pub
	volume := seedVolume(t, "Linked")
	studioID := seedStudio(t, "Studio")

	doPatch(t, router, "/studios/"+studioID+"/volumes", map[string]any{"volumeIds": []string{volume.ID}})

	event := next("catalog.events.volume.updated")
	if event.EntityID != volume.ID || event.Revision < 2 {
		t.Errorf("unexpected event: %+v", event)
	}
}

func TestEntityRestoreRollbackAndBulkPublishEvents(t *testing.T) {
	pub, next := captureEvents(t)
	router := newEventRouter(t, pub)
	studioID := seedStudio(t, "Restorable")

	if rec := doRequest(t, router, http.MethodDelete, "/studios/"+studioID, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d: %s", rec.Code, rec.Body.String())
	}
	assertEvent(t, next("catalog.events.studio.deleted"), studioID, 0)

	doPost(t, router, "/studios/"+studioID+"/restore", nil)
	restored := next("catalog.events.studio.updated")
	if restored.EntityID != studioID || restored.Revision < 1 {
		t.Errorf("unexpected restore event: %+v", restored)
	}

	doPost(t, router, "/studios/"+studioID+"/versions/"+strconv.Itoa(restored.Revision)+"/current", nil)
	assertEvent(t, next("catalog.events.studio.updated"), studioID, restored.Revision)

	rec := doPost(t, router, "/persons/bulk", []map[string]string{{"name": "Bulk Person"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk status = %d: %s", rec.Code, rec.Body.String())
	}
	if e := next("catalog.events.person.created"); e.EntityID == "" || e.Revision != 1 {
		t.Errorf("unexpected bulk event: %+v", e)
	}
}

// newEventRouter wires studio and person handlers with a live publisher. The handlers copy their
// config at setup, so the publisher has to be passed in rather than set afterward.
func newEventRouter(t *testing.T, pub *events.Publisher) *gin.Engine {
	t.Helper()
	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/profile" {
			_ = json.NewEncoder(w).Encode(map[string]string{"user_id": "auth0|test-reviewer"})
			return
		}
		_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: true, Roles: []string{authz.RoleAdmin}, Sub: "auth0|test-reviewer"})
	}))
	t.Cleanup(authAPI.Close)
	t.Cleanup(func() { studioVersionConfig.onPublishEvent, personVersionConfig.onPublishEvent = nil, nil })
	client := authz.NewClient(authAPI.URL, authAPI.URL)
	r := gin.New()
	setupStudioHandlers(r, persistence.NewInMemoryStore(0), cachettl.Config{}, client, pub)
	setupPersonHandlers(r, persistence.NewInMemoryStore(0), cachettl.Config{}, client, pub)
	return r
}
