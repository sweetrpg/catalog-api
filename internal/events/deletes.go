package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sweetrpg/common.go/logging"
)

const (
	systemCleanupDurable = "catalog-api-system-reference-cleanup"
	systemDeletedSubject = "gamesystems.events.system.deleted"
)

var (
	// deleteRetryDelay spaces out redelivery of a deletion that couldn't be applied.
	deleteRetryDelay = 30 * time.Second
	// ackProgressInterval is well under the default 30s ack wait, so a long removal isn't
	// redelivered while it's still running.
	ackProgressInterval = 10 * time.Second
)

// SystemDeletion is a game system that game-systems-api reports as deleted.
type SystemDeletion struct {
	ID        string
	SystemID  string
	DeletedAt time.Time
}

// SystemDeleteHandler applies one deletion. Returning an error redelivers the event after a
// delay; the handler MUST be idempotent.
type SystemDeleteHandler func(ctx context.Context, deletion SystemDeletion) error

// StartDeletes consumes system.deleted events on the reference-cleanup durable, binding in the
// background like Start.
func (c *Consumer) StartDeletes(handle SystemDeleteHandler) {
	if c == nil {
		return
	}
	c.consume(systemCleanupDurable, systemDeletedSubject, func(msg jetstream.Msg) {
		handleDelete(c.ctx, msg, handle)
	})
}

// handleDelete applies one delivery and acknowledges it only on success. A malformed event can
// never succeed, so it is logged and acknowledged.
func handleDelete(ctx context.Context, msg jetstream.Msg, handle SystemDeleteHandler) {
	deletion, eventID, ok := decodeSystemDelete(msg.Data())
	if !ok {
		_ = msg.Ack()
		return
	}
	stop := keepInProgress(msg)
	err := handle(ctx, deletion)
	stop()
	if err != nil {
		logging.Logger.Error("system reference cleanup failed, event will redeliver",
			"system_id", deletion.ID, "event_id", eventID, "error", err)
		_ = msg.NakWithDelay(deleteRetryDelay)
		return
	}
	_ = msg.Ack()
}

// keepInProgress extends msg's ack deadline until the returned function is called. The function
// waits for the sender, so no progress ack can follow the final one.
func keepInProgress(msg jetstream.Msg) (stop func()) {
	done, finished := make(chan struct{}), make(chan struct{})
	ticker := time.NewTicker(ackProgressInterval)
	go func() {
		defer close(finished)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = msg.InProgress()
			}
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}

func decodeSystemDelete(body []byte) (deletion SystemDeletion, eventID string, ok bool) {
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		logging.Logger.Error("system reference cleanup: undecodable envelope, dropping", "error", err)
		return deletion, "", false
	}
	if env.Action != "deleted" || env.EntityID == "" {
		logging.Logger.Error("system reference cleanup: not a system deletion, dropping",
			"event_id", env.EventID, "action", env.Action, "entity_id", env.EntityID)
		return deletion, env.EventID, false
	}
	var payload struct {
		SystemID  string    `json:"system_id"`
		DeletedAt time.Time `json:"deleted_at"`
	}
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			logging.Logger.Error("system reference cleanup: unreadable data, dropping", "event_id", env.EventID, "error", err)
			return deletion, env.EventID, false
		}
	}
	return SystemDeletion{ID: env.EntityID, SystemID: payload.SystemID, DeletedAt: payload.DeletedAt}, env.EventID, true
}
