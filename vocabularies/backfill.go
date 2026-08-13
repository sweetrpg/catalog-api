package vocabularies

import (
	"context"
	"fmt"

	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
)

// defaultFormats seeds the format vocabulary with a starting list, since submitters can never
// grow it themselves (format is editor/admin-only, see volume-format-selector's spec) and it
// otherwise starts out empty.
var defaultFormats = []string{
	"hardcover",
	"paperback",
	"PDF",
	"ePub",
	"VTT module",
	"boxed set",
}

// Backfill seeds the contribution-type vocabulary from every distinct role already present in
// the contributions collection, and seeds format with defaultFormats. Both operations are
// upsert-based (via Add) and safe to call on every startup - already-present values are a no-op.
func Backfill(ctx context.Context) error {
	roles, err := database.Db.Collection("contributions").Distinct(ctx, "roles", bson.D{})
	if err != nil {
		return fmt.Errorf("vocabularies: backfill contribution-type: distinct roles: %w", err)
	}
	for _, r := range roles {
		role, ok := r.(string)
		if !ok || role == "" {
			continue
		}
		if _, err := Add(ctx, TypeContributionType, role); err != nil {
			return fmt.Errorf("vocabularies: backfill contribution-type: %w", err)
		}
	}

	for _, f := range defaultFormats {
		if _, err := Add(ctx, TypeFormat, f); err != nil {
			return fmt.Errorf("vocabularies: backfill format: %w", err)
		}
	}

	return nil
}
