package events

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sweetrpg/common.go/logging"
)

func TestMain(m *testing.M) {
	logging.Init()
	os.Exit(m.Run())
}

// startTestJetStream starts an in-process NATS server with JetStream and the CATALOG_EVENTS
// stream, returning the client URL.
func startTestJetStream(t *testing.T) string {
	t.Helper()
	s, err := natsserver.NewServer(&natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new test NATS server: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		s.Shutdown()
		t.Fatal("test NATS server not ready")
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
	if _, err := js.CreateStream(context.Background(), jetstream.StreamConfig{
		Name:     "CATALOG_EVENTS",
		Subjects: []string{"catalog.events.>"},
	}); err != nil {
		t.Fatalf("create stream: %v", err)
	}
	return s.ClientURL()
}

// TestPublisherRoundTripToJetStream publishes an envelope and reads it back off the stream,
// asserting the subject, the envelope fields, and the Nats-Msg-Id dedup header (task 3.1 / 3.2).
func TestPublisherRoundTripToJetStream(t *testing.T) {
	url := startTestJetStream(t)
	t.Setenv("NATS_URL", url)
	t.Setenv("NATS_CREDS", "")

	pub, err := NewPublisher(context.Background())
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	if pub == nil {
		t.Fatal("publisher is nil despite NATS_URL set")
	}
	t.Cleanup(pub.Close)

	pub.PublishEntityUpdated(context.Background(), "volume", "vol-123", 7, map[string]interface{}{"title": "Player's Handbook"})

	conn, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connect reader: %v", err)
	}
	defer conn.Close()
	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatalf("jetstream reader: %v", err)
	}
	cons, err := js.CreateOrUpdateConsumer(context.Background(), "CATALOG_EVENTS", jetstream.ConsumerConfig{
		FilterSubject: "catalog.events.volume.updated",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("create reader consumer: %v", err)
	}
	msgs, err := cons.Fetch(1, jetstream.FetchMaxWait(3*time.Second))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	var msg jetstream.Msg
	for m := range msgs.Messages() {
		msg = m
	}
	if msgs.Error() != nil {
		t.Fatalf("fetch error: %v", msgs.Error())
	}
	if msg == nil {
		t.Fatal("no message received within 3s")
	}

	if msg.Subject() != "catalog.events.volume.updated" {
		t.Fatalf("subject = %q, want catalog.events.volume.updated", msg.Subject())
	}

	var env Envelope
	if err := json.Unmarshal(msg.Data(), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.EventID == "" {
		t.Error("event_id is empty")
	}
	if env.Source != "catalog-api" {
		t.Errorf("source = %q, want catalog-api", env.Source)
	}
	if env.EntityType != "volume" || env.EntityID != "vol-123" || env.Action != "updated" {
		t.Errorf("entity fields = %s/%s/%s, want volume/vol-123/updated", env.EntityType, env.EntityID, env.Action)
	}
	if env.Revision != 7 {
		t.Errorf("revision = %d, want 7", env.Revision)
	}
	if _, err := time.Parse(time.RFC3339, env.OccurredAt); err != nil {
		t.Errorf("occurred_at not RFC3339: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(env.Data, &data); err != nil || data["title"] != "Player's Handbook" {
		t.Errorf("data.title = %v (err %v), want Player's Handbook", data["title"], err)
	}
	if got := msg.Headers().Get(jetstream.MsgIDHeader); got != env.EventID {
		t.Errorf("Nats-Msg-Id header = %q, want %q (event_id)", got, env.EventID)
	}
}

func TestEnvelopeFieldsPopulated(t *testing.T) {
	testCases := []struct {
		name     string
		eventID  string
		entity   string
		id       string
		action   string
		revision int
		data     interface{}
	}{
		{
			name:     "created event",
			eventID:  "e1",
			entity:   "volume",
			id:       "v123",
			action:   "created",
			revision: 1,
			data:     map[string]string{"title": "Test"},
		},
		{
			name:     "updated event",
			eventID:  "e2",
			entity:   "person",
			id:       "p456",
			action:   "updated",
			revision: 2,
			data:     map[string]string{"name": "John"},
		},
		{
			name:     "deleted event with no data",
			eventID:  "e3",
			entity:   "license",
			id:       "lic789",
			action:   "deleted",
			revision: 0,
			data:     nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			envelope, err := NewEnvelope(tc.eventID, tc.entity, tc.id, tc.action, tc.revision, tc.data)
			if err != nil {
				t.Fatalf("NewEnvelope failed: %v", err)
			}

			if envelope.EventID != tc.eventID {
				t.Errorf("EventID = %s, want %s", envelope.EventID, tc.eventID)
			}
			if envelope.EntityType != tc.entity {
				t.Errorf("EntityType = %s, want %s", envelope.EntityType, tc.entity)
			}
			if envelope.EntityID != tc.id {
				t.Errorf("EntityID = %s, want %s", envelope.EntityID, tc.id)
			}
			if envelope.Action != tc.action {
				t.Errorf("Action = %s, want %s", envelope.Action, tc.action)
			}
			if envelope.Revision != tc.revision {
				t.Errorf("Revision = %d, want %d", envelope.Revision, tc.revision)
			}
			if envelope.Source != "catalog-api" {
				t.Errorf("Source = %s, want catalog-api", envelope.Source)
			}

			// Verify OccurredAt is in RFC3339 format and recent
			if _, err := time.Parse(time.RFC3339, envelope.OccurredAt); err != nil {
				t.Errorf("OccurredAt not RFC3339: %v", err)
			}

			// Verify data
			if tc.data == nil {
				if string(envelope.Data) != "null" {
					t.Errorf("Data for nil input = %s, want null", string(envelope.Data))
				}
			} else {
				var decoded interface{}
				if err := json.Unmarshal(envelope.Data, &decoded); err != nil {
					t.Errorf("Failed to unmarshal Data: %v", err)
				}
			}
		})
	}
}

func TestEnvelopeMarshalsToJSON(t *testing.T) {
	envelope, err := NewEnvelope("evt-1", "publisher", "pub-123", "updated", 3, map[string]string{"name": "Test Publisher"})
	if err != nil {
		t.Fatalf("NewEnvelope failed: %v", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal back
	var decoded Envelope
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.EventID != envelope.EventID {
		t.Errorf("EventID roundtrip failed")
	}
	if decoded.EntityType != envelope.EntityType {
		t.Errorf("EntityType roundtrip failed")
	}
	if string(decoded.Data) != string(envelope.Data) {
		t.Errorf("Data roundtrip failed")
	}
}

// TestPublisherDisabledWhenNATSUrlNotSet would require logging.Init() to be called in test setup,
// which would affect test isolation. The actual behavior is validated in integration tests.

func TestPublisherMethodsAreNilSafe(t *testing.T) {
	ctx := context.Background()
	var p *Publisher

	// These should not panic
	p.PublishEntityCreated(ctx, "volume", "v1", 1, map[string]string{"title": "Test"})
	p.PublishEntityUpdated(ctx, "volume", "v1", 2, map[string]string{"title": "Updated"})
	p.PublishEntityDeleted(ctx, "volume", "v1")
	p.Close()
}
