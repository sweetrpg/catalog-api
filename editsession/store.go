// Package editsession reads/deletes the shared, session-backed volume edit state that
// catalog-web writes to Redis (REDIS_DB=2 on catalog-api's own Redis instance - see
// docs/frontend-conventions.md's edit-session schema in sweetrpg/platform). catalog-api reads
// a session at finalize time and deletes it once finalize completes; the one exception to
// "catalog-api never creates a session" is pull-back (task 5.4), which recreates a pending
// proposal's diff as a fresh session so the submitter can resume editing it.
package editsession

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/sweetrpg/common.go/logging"
)

// KeyPrefix is the Redis key namespace for edit sessions: "edit-session:<userId>:<recordType>".
const KeyPrefix = "edit-session"

// Session mirrors the JSON schema catalog-web writes.
type Session struct {
	RecordID           string         `json:"recordId"`
	Fields             map[string]any `json:"fields"`
	StagedCoverAssetId string         `json:"stagedCoverAssetId,omitempty"`
	SampleAssetIds     []string       `json:"sampleAssetIds,omitempty"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
}

// Key returns the Redis key for a user's in-flight session on a given record type.
func Key(userID, recordType string) string {
	return fmt.Sprintf("%s:%s:%s", KeyPrefix, userID, recordType)
}

// Store reads/deletes edit sessions against a Redis pool already selecting REDIS_DB=2.
type Store struct {
	pool *redis.Pool
}

// NewStore wraps pool - the caller is responsible for the pool already dialing REDIS_DB=2.
func NewStore(pool *redis.Pool) *Store {
	return &Store{pool: pool}
}

// Get fetches a user's in-flight session for recordType, or nil if none exists.
func (s *Store) Get(ctx context.Context, userID, recordType string) (*Session, error) {
	logging.Logger.Debug("editsession.Get: enter", "userId", userID, "recordType", recordType)
	conn, err := s.pool.GetContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("editsession: get connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	raw, err := redis.Bytes(conn.Do("GET", Key(userID, recordType)))
	if err == redis.ErrNil {
		logging.Logger.Debug("editsession.Get: exit", "userId", userID, "recordType", recordType, "outcome", "miss")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("editsession: get %s: %w", Key(userID, recordType), err)
	}

	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("editsession: unmarshal %s: %w", Key(userID, recordType), err)
	}
	logging.Logger.Debug("editsession.Get: exit", "userId", userID, "recordType", recordType, "outcome", "hit", "recordId", session.RecordID)
	return &session, nil
}

// Set writes userID's session for recordType, overwriting any existing one. Used only by
// pull-back (task 5.4) - every other session write comes from catalog-web directly.
func (s *Store) Set(ctx context.Context, userID, recordType string, session Session) error {
	logging.Logger.Debug("editsession.Set: enter", "userId", userID, "recordType", recordType)
	conn, err := s.pool.GetContext(ctx)
	if err != nil {
		return fmt.Errorf("editsession: get connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	raw, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("editsession: marshal session for %s: %w", Key(userID, recordType), err)
	}
	if _, err := conn.Do("SET", Key(userID, recordType), raw); err != nil {
		return fmt.Errorf("editsession: set %s: %w", Key(userID, recordType), err)
	}
	logging.Logger.Debug("editsession.Set: exit", "userId", userID, "recordType", recordType, "outcome", "ok")
	return nil
}

// Delete removes a user's in-flight session for recordType. A missing session is not an error.
func (s *Store) Delete(ctx context.Context, userID, recordType string) error {
	logging.Logger.Debug("editsession.Delete: enter", "userId", userID, "recordType", recordType)
	conn, err := s.pool.GetContext(ctx)
	if err != nil {
		return fmt.Errorf("editsession: get connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Do("DEL", Key(userID, recordType)); err != nil {
		return fmt.Errorf("editsession: delete %s: %w", Key(userID, recordType), err)
	}
	logging.Logger.Debug("editsession.Delete: exit", "userId", userID, "recordType", recordType, "outcome", "ok")
	return nil
}
