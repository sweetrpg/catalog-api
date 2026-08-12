// Package submissioncap tracks the per-user override on how many pending (unapproved) proposed
// changes a submitter may have open at once - checked at volume edit session finalize time
// (durable-volume-editing task 5.1, sweetrpg/platform#38).
package submissioncap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const CollectionName = "submission_cap_overrides"

// DefaultCapEnvVar names the env var overriding defaultCap (the hardcoded fallback below).
const DefaultCapEnvVar = "SUBMISSION_CAP_DEFAULT"

// defaultCap is the unapproved-submission cap used when a user has no override and
// SUBMISSION_CAP_DEFAULT isn't set.
const defaultCap = 25

// override is one user's cap override document.
type override struct {
	UserID string `bson:"_id"`
	Cap    int    `bson:"cap"`
}

// Default returns the platform-wide default cap: SUBMISSION_CAP_DEFAULT if set and a valid
// non-negative integer, otherwise defaultCap.
func Default() int {
	raw, ok := os.LookupEnv(DefaultCapEnvVar)
	if !ok {
		return defaultCap
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return defaultCap
	}
	return parsed
}

// CapFor returns userID's effective cap: their override if one is set, otherwise Default().
func CapFor(ctx context.Context, userID string) (int, error) {
	o, err := GetOverride(ctx, userID)
	if err != nil {
		return 0, err
	}
	if o == nil {
		return Default(), nil
	}
	return *o, nil
}

// GetOverride returns userID's cap override, or nil if none is set.
func GetOverride(ctx context.Context, userID string) (*int, error) {
	var doc override
	err := database.Db.Collection(CollectionName).FindOne(ctx, bson.D{{Key: "_id", Value: userID}}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("submissioncap: get override for %s: %w", userID, err)
	}
	return &doc.Cap, nil
}

// SetOverride sets userID's cap override, replacing any existing one.
func SetOverride(ctx context.Context, userID string, capValue int) error {
	filter := bson.D{{Key: "_id", Value: userID}}
	update := bson.D{{Key: "$set", Value: override{UserID: userID, Cap: capValue}}}

	_, err := database.Db.Collection(CollectionName).UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("submissioncap: set override for %s: %w", userID, err)
	}
	return nil
}

// ClearOverride removes userID's cap override, falling them back to Default().
func ClearOverride(ctx context.Context, userID string) error {
	_, err := database.Db.Collection(CollectionName).DeleteOne(ctx, bson.D{{Key: "_id", Value: userID}})
	if err != nil {
		return fmt.Errorf("submissioncap: clear override for %s: %w", userID, err)
	}
	return nil
}
