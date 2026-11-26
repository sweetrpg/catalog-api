package sysdeletes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/sweetrpg/catalog-api/internal/events"
	"github.com/sweetrpg/common.go/logging"
)

func TestMain(m *testing.M) {
	logging.Init()
	os.Exit(m.Run())
}

// deletedAPI serves total deleted systems, answering failPage with a 500.
func deletedAPI(t *testing.T, total, failPage int) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		number, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if r.URL.Path != "/deleted-systems" || number == failPage || r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		systems := []map[string]any{}
		for i := (number - 1) * pageSize; i < min(number*pageSize, total); i++ {
			systems = append(systems, map[string]any{"id": fmt.Sprintf("s%d", i), "system_id": "slug", "deleted_at": "2026-11-26T00:00:00Z"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"systems": systems, "total": total})
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

func TestListReadsEveryPageWithoutACredential(t *testing.T) {
	for _, total := range []int{0, 1, pageSize, 2*pageSize + 1} {
		got, err := deletedAPI(t, total, 0).List(context.Background())
		if err != nil || len(got) != total {
			t.Errorf("total %d: got %d, err %v", total, len(got), err)
		}
	}
}

func TestListFailsWhenAnyPageFails(t *testing.T) {
	got, err := deletedAPI(t, 2*pageSize, 2).List(context.Background())
	if err == nil || got != nil {
		t.Errorf("got %d entries, err %v; want only an error", len(got), err)
	}
}

func ids(values ...string) []events.SystemDeletion {
	out := make([]events.SystemDeletion, len(values))
	for i, v := range values {
		out[i] = events.SystemDeletion{ID: v}
	}
	return out
}

func TestOnceAppliesListedAndRecordedDeletionsOnce(t *testing.T) {
	var applied []string
	err := Reconciler{
		List:  func(context.Context) ([]events.SystemDeletion, error) { return ids("a", "b"), nil },
		Known: func(context.Context) ([]events.SystemDeletion, error) { return ids("b", "c"), nil },
		Apply: func(_ context.Context, d events.SystemDeletion) error { applied = append(applied, d.ID); return nil },
	}.Once(context.Background())

	if err != nil || !slices.Equal(applied, []string{"a", "b", "c"}) {
		t.Errorf("applied %v, err %v", applied, err)
	}
}

func TestOnceKeepsGoingAfterFailures(t *testing.T) {
	var applied []string
	listErr, applyErr := errors.New("owner down"), errors.New("write failed")
	err := Reconciler{
		List:  func(context.Context) ([]events.SystemDeletion, error) { return nil, listErr },
		Known: func(context.Context) ([]events.SystemDeletion, error) { return ids("a", "b"), nil },
		Apply: func(_ context.Context, d events.SystemDeletion) error {
			applied = append(applied, d.ID)
			if d.ID == "a" {
				return applyErr
			}
			return nil
		},
	}.Once(context.Background())

	if !errors.Is(err, listErr) || !errors.Is(err, applyErr) || !slices.Equal(applied, []string{"a", "b"}) {
		t.Errorf("applied %v, err %v", applied, err)
	}
}

func TestRunReconcilesAtStartAndOnTheInterval(t *testing.T) {
	runs := make(chan struct{}, 10)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	none := func(context.Context) ([]events.SystemDeletion, error) { return nil, nil }
	go func() {
		defer close(done)
		Reconciler{
			List:  func(context.Context) ([]events.SystemDeletion, error) { runs <- struct{}{}; return nil, nil },
			Known: none,
		}.Run(ctx, 10*time.Millisecond)
	}()
	for range 3 {
		select {
		case <-runs:
		case <-time.After(5 * time.Second):
			t.Fatal("reconcile did not run")
		}
	}
	cancel()
	for {
		select {
		case <-runs:
		case <-done:
			return
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not return")
		}
	}
}

func TestParseInterval(t *testing.T) {
	for value, want := range map[string]time.Duration{
		"": DefaultInterval, "5m": 5 * time.Minute, "nonsense": DefaultInterval, "0s": DefaultInterval, "-1m": DefaultInterval,
	} {
		if got := ParseInterval(value); got != want {
			t.Errorf("ParseInterval(%q) = %v, want %v", value, got, want)
		}
	}
}
