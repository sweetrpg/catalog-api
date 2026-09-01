package events

import (
	"encoding/json"
	"time"
)

// Envelope is the JSON event payload published to NATS JetStream.
// Fields: event_id (UUID string), occurred_at (RFC3339), source ("catalog-api"),
// entity_type, entity_id, action, revision (entity's post-change version, 0 for delete),
// data (object; for volume.updated include at least the current title; may be empty for delete).
type Envelope struct {
	EventID    string          `json:"event_id"`
	OccurredAt string          `json:"occurred_at"`
	Source     string          `json:"source"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Action     string          `json:"action"`
	Revision   int             `json:"revision"`
	Data       json.RawMessage `json:"data"`
}

// NewEnvelope creates a new event envelope with all required fields.
// eventID should be a UUID string, occurredAt is RFC3339 formatted, revision is the
// entity's post-change version (0 for delete), and data is arbitrary JSON (may be null).
func NewEnvelope(eventID, entityType, entityID, action string, revision int, data interface{}) (*Envelope, error) {
	var rawData json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		rawData = b
	} else {
		rawData = json.RawMessage("null")
	}

	return &Envelope{
		EventID:    eventID,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		Source:     "catalog-api",
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Revision:   revision,
		Data:       rawData,
	}, nil
}
