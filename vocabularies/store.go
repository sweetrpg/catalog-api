package vocabularies

import (
	"context"
	"fmt"
	"sort"

	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureIndexes creates the unique (type, value) index Add relies on to make adding an
// already-present value a no-op rather than a duplicate. Safe to call on every startup.
func EnsureIndexes(ctx context.Context) error {
	_, err := database.Db.Collection(CollectionName).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "type", Value: 1}, {Key: "value", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("vocabularies: create index: %w", err)
	}
	return nil
}

// List returns every value stored for vocabType, sorted alphabetically.
func List(ctx context.Context, vocabType string) ([]string, error) {
	cursor, err := database.Db.Collection(CollectionName).Find(ctx, bson.D{{Key: "type", Value: vocabType}})
	if err != nil {
		return nil, fmt.Errorf("vocabularies: list %s: %w", vocabType, err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var entries []entry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("vocabularies: decode %s: %w", vocabType, err)
	}

	values := make([]string, len(entries))
	for i, e := range entries {
		values[i] = e.Value
	}
	sort.Strings(values)
	return values, nil
}

// Add upserts value into vocabType's list - a no-op if the value is already present. Returns
// whether the value was newly added.
func Add(ctx context.Context, vocabType, value string) (bool, error) {
	filter := bson.D{{Key: "type", Value: vocabType}, {Key: "value", Value: value}}
	update := bson.D{{Key: "$setOnInsert", Value: entry{Type: vocabType, Value: value}}}

	result, err := database.Db.Collection(CollectionName).UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return false, fmt.Errorf("vocabularies: add %s/%s: %w", vocabType, value, err)
	}
	return result.UpsertedCount > 0, nil
}
