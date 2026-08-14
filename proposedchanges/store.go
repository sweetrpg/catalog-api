package proposedchanges

import (
	"context"
	"fmt"
	"time"

	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// EnsureIndexes creates the compound index this package's queries rely on
// (record_type + record_id + status, for the per-record pending-list lookup). Safe to call on
// every startup - CreateOne is a no-op if an equivalent index already exists.
func EnsureIndexes(ctx context.Context) error {
	_, err := database.Db.Collection(CollectionName).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "record_type", Value: 1},
			{Key: "record_id", Value: 1},
			{Key: "status", Value: 1},
		},
	})
	if err != nil {
		return fmt.Errorf("proposedchanges: create index: %w", err)
	}
	return nil
}

// Add stores a new pending proposed change, stamping SubmittedAt, and returns its ID.
func Add(ctx context.Context, p *ProposedChange) (string, error) {
	p.Status = StatusPending
	p.SubmittedAt = time.Now()

	id, err := database.Insert[ProposedChange](CollectionName, *p)
	if err != nil {
		return "", fmt.Errorf("proposedchanges: insert: %w", err)
	}
	p.ID = id
	return id.Hex(), nil
}

// Get fetches a single proposed change by ID.
func Get(ctx context.Context, id string) (*ProposedChange, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("proposedchanges: invalid id %q: %w", id, err)
	}

	return database.Get[ProposedChange](CollectionName, oid)
}

// ListPending returns every pending proposed change for the given record, oldest first.
func ListPending(ctx context.Context, recordType, recordID string) ([]*ProposedChange, error) {
	filter := bson.D{
		{Key: "record_type", Value: recordType},
		{Key: "record_id", Value: recordID},
		{Key: "status", Value: StatusPending},
	}
	sort := bson.D{{Key: "submitted_at", Value: 1}}

	results, err := database.Query[ProposedChange](CollectionName, filter, sort, nil, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("proposedchanges: list pending: %w", err)
	}
	return results, nil
}

// ListPendingByType returns every pending proposed change across all records of one type,
// oldest first - used by the version-model migration (cmd/migrate-volumes) to find every
// still-pending proposal that needs to become a submitted version, not just one record's.
func ListPendingByType(ctx context.Context, recordType string) ([]*ProposedChange, error) {
	filter := bson.D{
		{Key: "record_type", Value: recordType},
		{Key: "status", Value: StatusPending},
	}
	sort := bson.D{{Key: "submitted_at", Value: 1}}

	results, err := database.Query[ProposedChange](CollectionName, filter, sort, nil, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("proposedchanges: list pending by type: %w", err)
	}
	return results, nil
}

// Update persists changes to an existing proposed change (its Diff field statuses, derived
// Status, and review metadata).
func Update(ctx context.Context, p *ProposedChange) error {
	_, _, err := database.Update[ProposedChange](CollectionName, p.ID, *p)
	if err != nil {
		return fmt.Errorf("proposedchanges: update: %w", err)
	}
	return nil
}

// CountPendingBySubmitter counts submittedBy's pending proposals across every record - the
// unapproved-submission-cap check at finalize time (task 5.1) is per-user, not per-record.
func CountPendingBySubmitter(ctx context.Context, submittedBy string) (int, error) {
	filter := bson.D{
		{Key: "submitted_by", Value: submittedBy},
		{Key: "status", Value: StatusPending},
	}

	count, err := database.Db.Collection(CollectionName).CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("proposedchanges: count pending by submitter: %w", err)
	}
	return int(count), nil
}
