package events

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sweetrpg/common.go/logging"
)

// ackRecorder records how a delivery was acknowledged. Other methods are not used.
type ackRecorder struct {
	jetstream.Msg
	body                     []byte
	acks, naks, progressAcks atomic.Int32
}

func (m *ackRecorder) Data() []byte                     { return m.body }
func (m *ackRecorder) Ack() error                       { m.acks.Add(1); return nil }
func (m *ackRecorder) NakWithDelay(time.Duration) error { m.naks.Add(1); return nil }
func (m *ackRecorder) InProgress() error                { m.progressAcks.Add(1); return nil }

func deletionBody(t *testing.T, action, entityID string, data any) []byte {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	body, err := json.Marshal(Envelope{EventID: "e1", EntityType: "system", EntityID: entityID, Action: action, Data: raw})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return body
}

func TestHandleDeleteAcksOnlyOnSuccess(t *testing.T) {
	logging.Init()
	valid := deletionBody(t, "deleted", "doc1", map[string]any{"system_id": "numenera", "deleted_at": "2026-11-26T00:00:00Z"})

	for _, fail := range []bool{false, true} {
		msg := &ackRecorder{body: valid}
		var got SystemDeletion
		handleDelete(context.Background(), msg, func(_ context.Context, d SystemDeletion) error {
			got = d
			if fail {
				return errors.New("database down")
			}
			return nil
		})
		wantAcks, wantNaks := int32(1), int32(0)
		if fail {
			wantAcks, wantNaks = 0, 1
		}
		if msg.acks.Load() != wantAcks || msg.naks.Load() != wantNaks {
			t.Errorf("fail=%v: acks %d naks %d", fail, msg.acks.Load(), msg.naks.Load())
		}
		if got.ID != "doc1" || got.SystemID != "numenera" || got.DeletedAt.IsZero() {
			t.Errorf("deletion = %+v", got)
		}
	}
}

// An event from a publisher that sends no data still identifies the system.
func TestDecodeSystemDeleteWithoutData(t *testing.T) {
	logging.Init()
	got, _, ok := decodeSystemDelete(deletionBody(t, "deleted", "doc1", nil))
	if !ok || got.ID != "doc1" || got.SystemID != "" {
		t.Errorf("got %+v, %v", got, ok)
	}
}

func TestHandleDeleteAcksMalformedEvents(t *testing.T) {
	logging.Init()
	bodies := map[string][]byte{
		"garbage":      []byte("not json"),
		"wrong action": deletionBody(t, "updated", "doc1", nil),
		"no entity":    deletionBody(t, "deleted", "", nil),
		"bad data":     deletionBody(t, "deleted", "doc1", "a string"),
	}
	for name, body := range bodies {
		msg := &ackRecorder{body: body}
		handleDelete(context.Background(), msg, func(context.Context, SystemDeletion) error {
			t.Errorf("%s: handler called", name)
			return nil
		})
		if msg.acks.Load() != 1 || msg.naks.Load() != 0 {
			t.Errorf("%s: acks %d naks %d, want it acked", name, msg.acks.Load(), msg.naks.Load())
		}
	}
}

func TestHandleDeleteSendsProgressWhileRunning(t *testing.T) {
	previous := ackProgressInterval
	ackProgressInterval = 5 * time.Millisecond
	t.Cleanup(func() { ackProgressInterval = previous })
	msg := &ackRecorder{body: deletionBody(t, "deleted", "doc1", nil)}

	handleDelete(context.Background(), msg, func(context.Context, SystemDeletion) error {
		time.Sleep(40 * time.Millisecond)
		return nil
	})

	sent := msg.progressAcks.Load()
	if sent == 0 {
		t.Fatal("no progress ack during a long handler")
	}
	time.Sleep(20 * time.Millisecond)
	if after := msg.progressAcks.Load(); after != sent {
		t.Errorf("progress acks continued after the final ack: %d -> %d", sent, after)
	}
}

// reservePort returns a free port with nothing listening on it.
func reservePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("free port: %v", err)
	}
	return port
}

func TestPublisherAndConsumersRecoverWhenBrokerStartsLater(t *testing.T) {
	logging.Init()
	previous := bindRetryDelay
	bindRetryDelay = 50 * time.Millisecond
	t.Cleanup(func() { bindRetryDelay = previous })
	port := reservePort(t)
	t.Setenv("NATS_URL", "nats://127.0.0.1:"+strconv.Itoa(port))
	t.Setenv("NATS_CREDS", "")
	t.Setenv("NATS_USER", "")

	publisher, err := NewPublisher(context.Background())
	if err != nil || publisher == nil {
		t.Fatalf("NewPublisher with the broker down = %v, %v", publisher, err)
	}
	t.Cleanup(publisher.Close)
	consumer, err := NewConsumer(context.Background())
	if err != nil || consumer == nil {
		t.Fatalf("NewConsumer with the broker down = %v, %v", consumer, err)
	}
	t.Cleanup(consumer.Stop)
	deletions := make(chan SystemDeletion, 1)
	titles := make(chan string, 1)
	consumer.StartDeletes(func(_ context.Context, d SystemDeletion) error { deletions <- d; return nil })
	consumer.Start(func(_ context.Context, _, title string) error { titles <- title; return nil })

	broker, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: port, JetStream: true, StoreDir: t.TempDir()})
	if err != nil {
		t.Fatalf("new NATS server: %v", err)
	}
	go broker.Start()
	t.Cleanup(broker.Shutdown)
	if !broker.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	for deadline := time.Now().Add(10 * time.Second); !publisher.conn.IsConnected(); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("publisher did not connect")
		}
	}
	for _, stream := range []jetstream.StreamConfig{
		{Name: systemEventsStream, Subjects: []string{"gamesystems.events.>"}},
		{Name: "CATALOG_EVENTS", Subjects: []string{"catalog.events.>"}},
	} {
		if _, err := publisher.js.CreateStream(context.Background(), stream); err != nil {
			t.Fatalf("create stream %s: %v", stream.Name, err)
		}
	}
	for subject, body := range map[string][]byte{
		systemDeletedSubject: deletionBody(t, "deleted", "doc1", nil),
		systemUpdatedSubject: deletionBody(t, "updated", "doc2", map[string]any{"title": "Renamed"}),
	} {
		if _, err := publisher.js.Publish(context.Background(), subject, body); err != nil {
			t.Fatalf("publish %s: %v", subject, err)
		}
	}

	timeout := time.After(30 * time.Second)
	for range 2 {
		select {
		case d := <-deletions:
			if d.ID != "doc1" {
				t.Errorf("deletion = %+v", d)
			}
		case title := <-titles:
			if title != "Renamed" {
				t.Errorf("title = %q", title)
			}
		case <-timeout:
			t.Fatal("consumers did not receive events after the broker started")
		}
	}

	publisher.PublishEntityDeleted(context.Background(), "volume", "v1")
	stream, err := publisher.js.Stream(context.Background(), "CATALOG_EVENTS")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if _, err := stream.GetLastMsgForSubject(context.Background(), "catalog.events.volume.deleted"); err != nil {
		t.Errorf("publisher event not stored: %v", err)
	}
}

func TestStopEndsBindingWhileBrokerIsDown(t *testing.T) {
	logging.Init()
	t.Setenv("NATS_URL", "nats://127.0.0.1:"+strconv.Itoa(reservePort(t)))
	consumer, err := NewConsumer(context.Background())
	if err != nil {
		t.Fatalf("NewConsumer: %v", err)
	}
	consumer.StartDeletes(func(context.Context, SystemDeletion) error { return nil })

	stopped := make(chan struct{})
	go func() { consumer.Stop(); close(stopped) }()

	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("Stop did not return")
	}
}
