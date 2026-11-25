package events

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// hangingBroker accepts publishes and never acknowledges them, so each one waits out its
// whole timeout. It returns a counter of the publishes that reached the broker.
func hangingBroker(t *testing.T) (url string, received func() int) {
	t.Helper()
	s, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: -1})
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
	sub, err := conn.SubscribeSync("catalog.events.>")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := conn.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return s.ClientURL(), func() int {
		n, _, err := sub.Pending()
		if err != nil {
			t.Fatalf("pending: %v", err)
		}
		return n
	}
}

func TestPublishBudgetBoundsTotalDelayPerRequest(t *testing.T) {
	url, received := hangingBroker(t)
	t.Setenv("NATS_URL", url)
	t.Setenv("NATS_CREDS", "")
	t.Setenv("PUBLISH_TIMEOUT_MS", "200")
	pub, err := NewPublisher(context.Background())
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	t.Cleanup(pub.Close)

	r := gin.New()
	r.Use(PublishBudget())
	r.POST("/loop", func(c *gin.Context) {
		for i := range 20 {
			pub.PublishEntityCreated(c.Request.Context(), "person", strconv.Itoa(i), 1, nil)
		}
	})

	// The budget is per request: the second request gets its own.
	for request := 1; request <= 2; request++ {
		start := time.Now()
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/loop", nil))
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Errorf("request %d took %v, want about one 200ms budget, not 20", request, elapsed)
		}
		time.Sleep(50 * time.Millisecond)
		if got := received(); got != request {
			t.Errorf("broker received %d publishes after request %d, want %d", got, request, request)
		}
	}
}

// slowBroker acknowledges every publish after delay. It returns a counter of the publishes it
// acknowledged.
func slowBroker(t *testing.T, delay time.Duration) (url string, acked func() int) {
	t.Helper()
	s, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: -1})
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
	var count atomic.Int64
	if _, err := conn.Subscribe("catalog.events.>", func(m *nats.Msg) {
		time.Sleep(delay)
		count.Add(1)
		if err := m.Respond([]byte(`{"stream":"catalog","seq":1}`)); err != nil {
			t.Errorf("ack: %v", err)
		}
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := conn.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return s.ClientURL(), func() int { return int(count.Load()) }
}

func TestPublishBudgetIgnoresSuccessfulPublishes(t *testing.T) {
	url, acked := slowBroker(t, 20*time.Millisecond)
	t.Setenv("NATS_URL", url)
	t.Setenv("NATS_CREDS", "")
	t.Setenv("PUBLISH_TIMEOUT_MS", "200")
	pub, err := NewPublisher(context.Background())
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	t.Cleanup(pub.Close)

	r := gin.New()
	r.Use(PublishBudget())
	r.POST("/loop", func(c *gin.Context) {
		for i := range 20 {
			pub.PublishEntityCreated(c.Request.Context(), "person", strconv.Itoa(i), 1, nil)
		}
	})

	start := time.Now()
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/loop", nil))

	if elapsed := time.Since(start); elapsed <= 200*time.Millisecond {
		t.Fatalf("publishes took %v, need more than the 200ms budget to prove anything", elapsed)
	}
	if got := acked(); got != 20 {
		t.Errorf("broker acknowledged %d publishes, want all 20", got)
	}
}
