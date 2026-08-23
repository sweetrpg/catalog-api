package editsession

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
	"github.com/sweetrpg/common.go/logging"
)

func TestMain(m *testing.M) {
	logging.Init()
	os.Exit(m.Run())
}

func newTestPool(t *testing.T) (*redis.Pool, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", mr.Addr())
		},
	}
	t.Cleanup(func() { _ = pool.Close() })

	return pool, mr
}

func TestGetReturnsNilWhenNoSession(t *testing.T) {
	pool, _ := newTestPool(t)
	store := NewStore(pool)

	session, err := store.Get(context.Background(), "user-1", "volume")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if session != nil {
		t.Errorf("Get() = %+v, want nil", session)
	}
}

func TestGetDecodesSessionWrittenDirectly(t *testing.T) {
	pool, mr := newTestPool(t)
	store := NewStore(pool)

	want := Session{
		RecordID:           "volume-1",
		Fields:             map[string]any{"title": "Staged Title"},
		StagedCoverAssetId: "cover-abc",
		SampleAssetIds:     []string{"sample-1"},
		CreatedAt:          time.Now().UTC().Truncate(time.Second),
		UpdatedAt:          time.Now().UTC().Truncate(time.Second),
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	if err := mr.Set(Key("user-1", "volume"), string(raw)); err != nil {
		t.Fatalf("seed redis key: %v", err)
	}

	got, err := store.Get(context.Background(), "user-1", "volume")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil {
		t.Fatal("Get() = nil, want the seeded session")
	}
	if got.RecordID != want.RecordID || got.StagedCoverAssetId != want.StagedCoverAssetId {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}

func TestDeleteRemovesSession(t *testing.T) {
	pool, mr := newTestPool(t)
	store := NewStore(pool)

	if err := mr.Set(Key("user-1", "volume"), `{"recordId":"volume-1"}`); err != nil {
		t.Fatalf("seed redis key: %v", err)
	}

	if err := store.Delete(context.Background(), "user-1", "volume"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if mr.Exists(Key("user-1", "volume")) {
		t.Error("key still exists after Delete()")
	}
}

func TestDeleteOfMissingSessionIsNotAnError(t *testing.T) {
	pool, _ := newTestPool(t)
	store := NewStore(pool)

	if err := store.Delete(context.Background(), "user-1", "volume"); err != nil {
		t.Fatalf("Delete() error = %v, want nil for a missing key", err)
	}
}

func TestKeyFormat(t *testing.T) {
	if got, want := Key("user-1", "volume"), "edit-session:user-1:volume"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}
