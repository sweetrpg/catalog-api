package events

import (
	"os"

	"github.com/nats-io/nats.go"
)

// connectOptions builds the options publishers and consumers share. Without retry-on-failed-
// connect a broker that is down at startup disables events until a restart, and by default the
// client gives up reconnecting after a long outage. Serving never waits for the broker.
func connectOptions() []nats.Option {
	opts := []nats.Option{nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1)}
	if creds := os.Getenv("NATS_CREDS"); creds != "" {
		opts = append(opts, nats.UserCredentials(creds))
	} else if user := os.Getenv("NATS_USER"); user != "" {
		opts = append(opts, nats.UserInfo(user, os.Getenv("NATS_PASSWORD")))
	}
	return opts
}
