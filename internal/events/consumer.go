package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

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

// bindRetryDelay is how long to wait before retrying a durable that couldn't be bound.
var bindRetryDelay = 5 * time.Second

// SystemUpdateHandler applies one gamesystems.events.system.updated event. Returning an error
// Naks the message so JetStream redelivers it; the handler MUST be idempotent.
type SystemUpdateHandler func(ctx context.Context, systemID, title string) error

// Consumer binds catalog-api's durable pull consumers on the game-systems events stream.
type Consumer struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	consumes []jetstream.ConsumeContext
	binding  sync.WaitGroup
}

// NewConsumer builds a Consumer from the environment (same NATS_URL / NATS_CREDS /
// NATS_USER+NATS_PASSWORD scheme as the publisher). Returns (nil, nil) when NATS_URL is unset so
// the caller treats sync as disabled rather than a startup error. An unreachable broker is not
// an error: the connection keeps retrying.
func NewConsumer(ctx context.Context) (*Consumer, error) {
	natsURL := util.GetEnv("NATS_URL", "")
	if natsURL == "" {
		logging.Logger.Warn("NATS_URL not set; game system event consumers disabled")
		return nil, nil
	}

	conn, err := nats.Connect(natsURL, connectOptions()...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Consumer{conn: conn, js: js, ctx: ctx, cancel: cancel}, nil
}

// Start consumes system.updated events on the title-sync durable. It returns at once and binds
// in the background, retrying until the broker and stream are available.
func (c *Consumer) Start(handle SystemUpdateHandler) {
	if c == nil {
		return
	}
	c.consume(systemTitleSyncDurable, systemUpdatedSubject, func(msg jetstream.Msg) {
		if herr := dispatchSystemUpdate(c.ctx, msg, handle); herr != nil {
			logging.Logger.Error("system-title sync: handler failed, event will redeliver",
				"subject", msg.Subject(), "error", herr)
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
}

// consume binds durable in the background and delivers its messages to receive.
func (c *Consumer) consume(durable, subject string, receive jetstream.MessageHandler) {
	c.binding.Add(1)
	go func() {
		defer c.binding.Done()
		for {
			err := c.bind(durable, subject, receive)
			if err == nil {
				logging.Logger.Info("consumer bound", "stream", systemEventsStream, "durable", durable)
				return
			}
			logging.Logger.Warn("consumer not bound, will retry", "durable", durable, "error", err)
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(bindRetryDelay):
			}
		}
	}()
}

// bind attaches to the durable (declared as a NACK CRD in sweetrpg/infrastructure; created here
// as a fallback for local runs without NACK) and starts delivery.
func (c *Consumer) bind(durable, subject string, receive jetstream.MessageHandler) error {
	cons, err := c.js.Consumer(c.ctx, systemEventsStream, durable)
	if errors.Is(err, jetstream.ErrConsumerNotFound) {
		cons, err = c.js.CreateOrUpdateConsumer(c.ctx, systemEventsStream, jetstream.ConsumerConfig{
			Durable:       durable,
			FilterSubject: subject,
			AckPolicy:     jetstream.AckExplicitPolicy,
		})
	}
	if err != nil {
		return err
	}
	cc, err := cons.Consume(receive)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consumes = append(c.consumes, cc)
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
	c.cancel()
	c.binding.Wait()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, cc := range c.consumes {
		cc.Stop()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
