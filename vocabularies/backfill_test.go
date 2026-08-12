package vocabularies

import (
	"context"
	"testing"

	"github.com/sweetrpg/mongodb.go/database"
)

func TestBackfillSeedsContributionTypesFromCredits(t *testing.T) {
	ctx := context.Background()

	_, err := database.Db.Collection("contributions").InsertOne(ctx, map[string]any{
		"_id":       "contribution-backfill-test",
		"person_id": "person-1",
		"volume_id": "volume-1",
		"roles":     []string{"Backfill Test Role"},
	})
	if err != nil {
		t.Fatalf("seed contribution: %v", err)
	}

	if err := Backfill(ctx); err != nil {
		t.Fatalf("Backfill() error = %v", err)
	}

	values, err := List(ctx, TypeContributionType)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !contains(values, "Backfill Test Role") {
		t.Errorf("contribution-type List() = %v, want to contain %q", values, "Backfill Test Role")
	}

	formats, err := List(ctx, TypeFormat)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !contains(formats, "hardcover") {
		t.Errorf("format List() = %v, want to contain %q", formats, "hardcover")
	}
}
