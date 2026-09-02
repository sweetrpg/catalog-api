package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
)

const (
	systemEventsStream     = "GAMESYSTEMS_EVENTS"
	systemTitleSyncDurable = "catalog-api-system-title-sync"
	systemUpdatedSubject   = "gamesystems.events.system.updated"
)

// SystemUpdateHandler applies one gamesystems.events.system.updated event. Returning an error
// Naks the message so JetStream redelivers it; the handler MUST be idempotent.
type SystemUpdateHandler func(ctx context.Context, systemID, title string) error

// Consumer binds catalog-api's durable pull consumer on the game-systems events stream and
// dispatches system.updated events to a handler.
type Consumer struct {
	conn *nats.Conn
	js   jetstream.JetStream
	cc   jetstream.ConsumeContext
}

// NewConsumer builds a Consumer from the environment (same NATS_URL / NATS_CREDS /
// NATS_USER+NATS_PASSWORD scheme as the publisher). Returns (nil, nil) when NATS_URL is unset so
// the caller treats sync as disabled rather than a startup error.
func NewConsumer(ctx context.Context) (*Consumer, error) {
	natsURL := util.GetEnv("NATS_URL", "")
	if natsURL == "" {
		logging.Logger.Warn("NATS_URL not set; system-title sync consumer disabled")
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
	return &Consumer{conn: conn, js: js}, nil
}

// Start binds the durable (declared as a NACK CRD in sweetrpg/infrastructure; created here as a
// fallback for local runs without NACK) and begins delivering events to handle in the
// background. It returns once the subscription is established.
func (c *Consumer) Start(ctx context.Context, handle SystemUpdateHandler) error {
	if c == nil {
		return nil
	}

	cons, err := c.js.Consumer(ctx, systemEventsStream, systemTitleSyncDurable)
	if errors.Is(err, jetstream.ErrConsumerNotFound) {
		cons, err = c.js.CreateOrUpdateConsumer(ctx, systemEventsStream, jetstream.ConsumerConfig{
			Durable:       systemTitleSyncDurable,
			FilterSubject: systemUpdatedSubject,
			AckPolicy:     jetstream.AckExplicitPolicy,
		})
	}
	if err != nil {
		return fmt.Errorf("bind consumer %s/%s: %w", systemEventsStream, systemTitleSyncDurable, err)
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		if herr := dispatchSystemUpdate(ctx, msg, handle); herr != nil {
			logging.Logger.Error("system-title sync: handler failed, event will redeliver",
				"subject", msg.Subject(), "error", herr)
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}
	c.cc = cc
	logging.Logger.Info("system-title sync consumer bound", "stream", systemEventsStream, "durable", systemTitleSyncDurable)
	return nil
}

// dispatchSystemUpdate decodes the message and invokes handle. A malformed envelope is dropped
// (returns nil -> Ack) rather than poisoning the durable; a handler error propagates (-> Nak).
func dispatchSystemUpdate(ctx context.Context, msg jetstream.Msg, handle SystemUpdateHandler) error {
	systemID, title, ok := decodeSystemUpdate(msg.Data())
	if !ok {
		return nil
	}
	return handle(ctx, systemID, title)
}

// decodeSystemUpdate parses a raw event body into (systemID, title). ok is false for an
// undecodable envelope or one that is not a system.updated with a non-empty entity_id - the
// caller acks and moves on in that case.
func decodeSystemUpdate(body []byte) (systemID, title string, ok bool) {
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		logging.Logger.Error("system-title sync: undecodable envelope, dropping", "error", err)
		return "", "", false
	}
	if env.Action != "updated" || env.EntityID == "" {
		return "", "", false
	}
	var payload struct {
		Title string `json:"title"`
	}
	_ = json.Unmarshal(env.Data, &payload)
	return env.EntityID, payload.Title, true
}

// Stop halts delivery and closes the connection.
func (c *Consumer) Stop() {
	if c == nil {
		return
	}
	if c.cc != nil {
		c.cc.Stop()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
