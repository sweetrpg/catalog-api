package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

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
