package cachettl

import (
	"os"
	"testing"
	"time"

	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/common.go/logging"
)

func TestMain(m *testing.M) {
	// Package logging's zero-value Logger blocks on any call until Init() runs - Load() logs
	// warnings on malformed input, so tests need a real logger.
	logging.Init()
	os.Exit(m.Run())
}

func TestTTLReturnsExplicitEntry(t *testing.T) {
	t.Setenv(constants.CACHE_TTLS, "licenses=30m,volumes=15m")
	cfg := Load()

	if got := cfg.TTL("licenses"); got != 30*time.Minute {
		t.Fatalf("TTL(licenses) = %v, want 30m", got)
	}
	if got := cfg.TTL("volumes"); got != 15*time.Minute {
		t.Fatalf("TTL(volumes) = %v, want 15m", got)
	}
}

func TestTTLFallsBackToDefaultForUnlistedRoute(t *testing.T) {
	t.Setenv(constants.CACHE_TTLS, "licenses=30m")
	cfg := Load()

	if got := cfg.TTL("reviews"); got != defaultTTLFallback {
		t.Fatalf("TTL(reviews) = %v, want fallback default %v", got, defaultTTLFallback)
	}
}

func TestTTLUsesConfiguredDefault(t *testing.T) {
	t.Setenv(constants.CACHE_TTLS, "")
	t.Setenv(constants.CACHE_DEFAULT_TTL, "5m")
	cfg := Load()

	if got := cfg.TTL("anything"); got != 5*time.Minute {
		t.Fatalf("TTL(anything) = %v, want 5m", got)
	}
}

func TestLoadSkipsMalformedEntries(t *testing.T) {
	t.Setenv(constants.CACHE_TTLS, "licenses=30m,not-a-valid-entry,reviews=notaduration,studios=10m")
	cfg := Load()

	if got := cfg.TTL("licenses"); got != 30*time.Minute {
		t.Fatalf("TTL(licenses) = %v, want 30m", got)
	}
	if got := cfg.TTL("studios"); got != 10*time.Minute {
		t.Fatalf("TTL(studios) = %v, want 10m", got)
	}
	if got := cfg.TTL("reviews"); got != defaultTTLFallback {
		t.Fatalf("TTL(reviews) = %v, want fallback default (malformed entry skipped)", got)
	}
}

func TestLoadSkipsInvalidDefaultTTL(t *testing.T) {
	t.Setenv(constants.CACHE_DEFAULT_TTL, "not-a-duration")
	cfg := Load()

	if got := cfg.TTL("anything"); got != defaultTTLFallback {
		t.Fatalf("TTL(anything) = %v, want fallback default %v", got, defaultTTLFallback)
	}
}
