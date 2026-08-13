package submissioncap

import (
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

func TestDefaultFallsBackWithoutEnvVar(t *testing.T) {
	_ = os.Unsetenv(DefaultCapEnvVar)
	if got := Default(); got != defaultCap {
		t.Errorf("Default() = %d, want %d", got, defaultCap)
	}
}

func TestDefaultReadsEnvVar(t *testing.T) {
	t.Setenv(DefaultCapEnvVar, "10")
	if got := Default(); got != 10 {
		t.Errorf("Default() = %d, want 10", got)
	}
}

func TestDefaultIgnoresInvalidEnvVar(t *testing.T) {
	t.Setenv(DefaultCapEnvVar, "not-a-number")
	if got := Default(); got != defaultCap {
		t.Errorf("Default() = %d, want %d (fallback on invalid value)", got, defaultCap)
	}
}

func TestCapForFallsBackToDefaultWithoutOverride(t *testing.T) {
	_ = os.Unsetenv(DefaultCapEnvVar)
	ctx := t.Context()

	got, err := CapFor(ctx, "user-no-override")
	if err != nil {
		t.Fatalf("CapFor() error = %v", err)
	}
	if got != defaultCap {
		t.Errorf("CapFor() = %d, want %d", got, defaultCap)
	}
}

func TestSetOverrideChangesOnlyThatUser(t *testing.T) {
	ctx := t.Context()

	if err := SetOverride(ctx, "user-override-1", 5); err != nil {
		t.Fatalf("SetOverride() error = %v", err)
	}

	got, err := CapFor(ctx, "user-override-1")
	if err != nil {
		t.Fatalf("CapFor() error = %v", err)
	}
	if got != 5 {
		t.Errorf("CapFor(user-override-1) = %d, want 5", got)
	}

	other, err := CapFor(ctx, "user-unaffected")
	if err != nil {
		t.Fatalf("CapFor() error = %v", err)
	}
	if other != Default() {
		t.Errorf("CapFor(user-unaffected) = %d, want %d (unaffected by another user's override)", other, Default())
	}
}

func TestClearOverrideRestoresDefault(t *testing.T) {
	ctx := t.Context()

	if err := SetOverride(ctx, "user-override-2", 3); err != nil {
		t.Fatalf("SetOverride() error = %v", err)
	}
	if err := ClearOverride(ctx, "user-override-2"); err != nil {
		t.Fatalf("ClearOverride() error = %v", err)
	}

	got, err := CapFor(ctx, "user-override-2")
	if err != nil {
		t.Fatalf("CapFor() error = %v", err)
	}
	if got != Default() {
		t.Errorf("CapFor() after clear = %d, want %d", got, Default())
	}
}
