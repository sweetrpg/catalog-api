package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
)

// Publisher publishes entity-change events to NATS JetStream.
type Publisher struct {
	conn           *nats.Conn
	js             jetstream.JetStream
	publishTimeout time.Duration
}

// NewPublisher creates a new NATS JetStream publisher from environment configuration.
// NATS_URL: NATS server URL (e.g., "nats://localhost:4222")
// NATS_CREDS: path to credentials file (optional, empty/unset for no-auth)
// PUBLISH_TIMEOUT_MS: milliseconds to wait for publish (default 3000)
func NewPublisher(ctx context.Context) (*Publisher, error) {
	natsURL := util.GetEnv("NATS_URL", "")
	if natsURL == "" {
		logging.Logger.Warn("NATS_URL not set; event publishing disabled")
		return nil, nil
	}

	opts := []nats.Option{}
	if creds := os.Getenv("NATS_CREDS"); creds != "" {
		opts = append(opts, nats.UserCredentials(creds))
	} else if user := os.Getenv("NATS_USER"); user != "" {
		opts = append(opts, nats.UserInfo(user, os.Getenv("NATS_PASSWORD")))
	}

	conn, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}

	timeoutMs := util.GetEnvInt("PUBLISH_TIMEOUT_MS", 3000)

	p := &Publisher{
		conn:           conn,
		js:             js,
		publishTimeout: time.Duration(timeoutMs) * time.Millisecond,
	}

	logging.Logger.Info("Publisher initialized", "nats_url", natsURL, "publish_timeout_ms", timeoutMs)
	return p, nil
}

// Close closes the NATS connection.
func (p *Publisher) Close() {
	if p != nil && p.conn != nil {
		p.conn.Close()
	}
}

// PublishEntityCreated publishes a created event for an entity.
// data is the entity's current state.
func (p *Publisher) PublishEntityCreated(ctx context.Context, entityType, entityID string, revision int, data interface{}) {
	if p == nil || p.conn == nil {
		return
	}

	eventID := uuid.NewString()
	envelope, err := NewEnvelope(eventID, entityType, entityID, "created", revision, data)
	if err != nil {
		logging.Logger.Error("PublishEntityCreated: envelope creation failed", "entity_type", entityType, "entity_id", entityID, "error", err)
		return
	}

	p.publishWithFallback(ctx, entityType, "created", eventID, envelope)
}

// PublishEntityUpdated publishes an updated event for an entity.
// data is the entity's current state (for volume.updated must include title).
func (p *Publisher) PublishEntityUpdated(ctx context.Context, entityType, entityID string, revision int, data interface{}) {
	if p == nil || p.conn == nil {
		return
	}

	eventID := uuid.NewString()
	envelope, err := NewEnvelope(eventID, entityType, entityID, "updated", revision, data)
	if err != nil {
		logging.Logger.Error("PublishEntityUpdated: envelope creation failed", "entity_type", entityType, "entity_id", entityID, "error", err)
		return
	}

	p.publishWithFallback(ctx, entityType, "updated", eventID, envelope)
}

// PublishEntityDeleted publishes a deleted event for an entity.
// revision is 0 for delete, data may be empty.
func (p *Publisher) PublishEntityDeleted(ctx context.Context, entityType, entityID string) {
	if p == nil || p.conn == nil {
		return
	}

	eventID := uuid.NewString()
	envelope, err := NewEnvelope(eventID, entityType, entityID, "deleted", 0, nil)
	if err != nil {
		logging.Logger.Error("PublishEntityDeleted: envelope creation failed", "entity_type", entityType, "entity_id", entityID, "error", err)
		return
	}

	p.publishWithFallback(ctx, entityType, "deleted", eventID, envelope)
}

// publishWithFallback publishes the envelope with a bounded timeout and fails open:
// on error/timeout/unreachable broker, logs the dropped event and returns success.
func (p *Publisher) publishWithFallback(ctx context.Context, entityType, action, eventID string, envelope *Envelope) {
	subject := fmt.Sprintf("catalog.events.%s.%s", entityType, action)

	body, err := json.Marshal(envelope)
	if err != nil {
		logging.Logger.Error("publishWithFallback: marshal failed", "entity_type", entityType, "entity_id", envelope.EntityID, "action", action, "error", err)
		return
	}

	// Bounded publish timeout
	ctx, cancel := context.WithTimeout(ctx, p.publishTimeout)
	defer cancel()

	// ponytail: fail-open on timeout/error. Alternative: in-process queue with goroutine retry,
	// but unbounded growth + restarts lose queued events. No retry is correct here.
	_, err = p.js.Publish(ctx, subject, body, jetstream.WithMsgID(eventID))
	if err != nil {
		logging.Logger.Error("PublishEntityEvent: publish failed (event dropped)", "subject", subject, "entity_id", envelope.EntityID, "event_id", eventID, "error", err)
		return
	}

	logging.Logger.Info("PublishEntityEvent: published", "subject", subject, "entity_id", envelope.EntityID, "event_id", eventID)
}
