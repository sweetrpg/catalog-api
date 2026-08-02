// Package cachettl resolves the cache TTL for a given route group, loaded once at startup
// from the CACHE_TTLS env var so slower-changing entities and faster-changing ones aren't
// forced onto the same cache policy.
package cachettl

import (
	"strings"
	"time"

	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
)

const defaultTTLFallback = time.Hour

// Config resolves a per-route-group TTL, falling back to a configured default for any route
// group without an explicit entry.
type Config struct {
	ttls       map[string]time.Duration
	defaultTTL time.Duration
}

// Load parses CACHE_TTLS ("route=duration,route=duration", e.g. "licenses=30m,volumes=15m")
// and CACHE_DEFAULT_TTL (a single duration string) from the environment. Malformed entries are
// logged and skipped rather than failing startup.
func Load() Config {
	defaultTTL := defaultTTLFallback
	if raw := util.GetEnv(constants.CACHE_DEFAULT_TTL, ""); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			defaultTTL = d
		} else {
			logging.Logger.Warn("Invalid CACHE_DEFAULT_TTL, using fallback default",
				"value", raw, "fallback", defaultTTLFallback.String(), "error", err.Error())
		}
	}

	ttls := map[string]time.Duration{}
	raw := util.GetEnv(constants.CACHE_TTLS, "")
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		route, value, found := strings.Cut(entry, "=")
		if !found {
			logging.Logger.Warn("Invalid CACHE_TTLS entry, expected route=duration", "entry", entry)
			continue
		}
		d, err := time.ParseDuration(strings.TrimSpace(value))
		if err != nil {
			logging.Logger.Warn("Invalid CACHE_TTLS duration, skipping entry",
				"entry", entry, "error", err.Error())
			continue
		}
		ttls[strings.TrimSpace(route)] = d
	}

	return Config{ttls: ttls, defaultTTL: defaultTTL}
}

// TTL returns the configured TTL for the given route group, or the configured default if the
// route group has no explicit entry.
func (c Config) TTL(route string) time.Duration {
	if d, ok := c.ttls[route]; ok {
		return d
	}
	return c.defaultTTL
}
