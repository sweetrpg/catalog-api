package vocabularies

import (
	"context"
	"os"
	"testing"

	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/constants"
	"github.com/sweetrpg/mongodb.go/database"
)

func TestMain(m *testing.M) {
	if os.Getenv("TEST_DB_URI") == "" {
		os.Exit(0)
	}
	_ = os.Setenv(constants.DB_URI, os.Getenv("TEST_DB_URI"))
	logging.Init()
	database.SetupDatabase()
	os.Exit(m.Run())
}

func TestAddAndList(t *testing.T) {
	ctx := context.Background()

	added, err := Add(ctx, TypePropertyName, "ISBN")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !added {
		t.Error("Add() = false on first insert, want true")
	}

	added, err = Add(ctx, TypePropertyName, "ISBN")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if added {
		t.Error("Add() = true on duplicate insert, want false (no-op)")
	}

	values, err := List(ctx, TypePropertyName)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	found := false
	for _, v := range values {
		if v == "ISBN" {
			found = true
		}
	}
	if !found {
		t.Errorf("List() = %v, want to contain %q", values, "ISBN")
	}
}

func TestListIsScopedByType(t *testing.T) {
	ctx := context.Background()

	if _, err := Add(ctx, TypeContributionType, "Illustrator"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := Add(ctx, TypePropertyName, "Illustrator"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	contributionValues, err := List(ctx, TypeContributionType)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	propertyValues, err := List(ctx, TypePropertyName)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if !contains(contributionValues, "Illustrator") {
		t.Errorf("contribution-type List() = %v, want to contain %q", contributionValues, "Illustrator")
	}
	if !contains(propertyValues, "Illustrator") {
		t.Errorf("property-name List() = %v, want to contain %q", propertyValues, "Illustrator")
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func TestIsValidType(t *testing.T) {
	cases := map[string]bool{
		TypeContributionType: true,
		TypePropertyName:     true,
		TypeFormat:           true,
		"nonsense":           false,
		"":                   false,
	}
	for input, want := range cases {
		if got := IsValidType(input); got != want {
			t.Errorf("IsValidType(%q) = %v, want %v", input, got, want)
		}
	}
}
