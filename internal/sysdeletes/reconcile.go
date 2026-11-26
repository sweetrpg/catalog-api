package sysdeletes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sweetrpg/catalog-api/internal/events"
	"github.com/sweetrpg/common.go/logging"
)

// DefaultInterval is how often deletions are reconciled unless configured otherwise.
const DefaultInterval = 15 * time.Minute

// ParseInterval reads a Go duration. An empty, invalid, or non-positive value means
// DefaultInterval.
func ParseInterval(value string) time.Duration {
	if value == "" {
		return DefaultInterval
	}
	interval, err := time.ParseDuration(value)
	if err != nil || interval <= 0 {
		logging.Logger.Warn("invalid reconcile interval, using the default", "value", value, "default", DefaultInterval)
		return DefaultInterval
	}
	return interval
}

// Reconciler applies every known deletion.
type Reconciler struct {
	// List reads the owner's deleted systems.
	List func(context.Context) ([]events.SystemDeletion, error)
	// Known returns the deletions catalog has already recorded.
	Known func(context.Context) ([]events.SystemDeletion, error)
	// Apply records a deletion and removes its references. It must be idempotent.
	Apply events.SystemDeleteHandler
}

// Once applies the union of the listed and recorded deletions. Recorded ones are reapplied
// every time, which removes a reference written by a request that raced its tombstone. A failed
// list request doesn't stop that, and one failed deletion doesn't stop the others.
func (r Reconciler) Once(ctx context.Context) error {
	var failures []error
	listed, err := r.List(ctx)
	if err != nil {
		failures = append(failures, err)
	}
	known, err := r.Known(ctx)
	if err != nil {
		failures = append(failures, fmt.Errorf("read recorded deletions: %w", err))
	}
	seen := map[string]bool{}
	for _, deletion := range append(listed, known...) {
		if seen[deletion.ID] {
			continue
		}
		seen[deletion.ID] = true
		if err := r.Apply(ctx, deletion); err != nil {
			failures = append(failures, fmt.Errorf("system %s: %w", deletion.ID, err))
		}
	}
	return errors.Join(failures...)
}

// Run reconciles now and then every interval until ctx ends.
func (r Reconciler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := r.Once(ctx); err != nil {
			logging.Logger.Warn("game system deletion reconcile incomplete, will retry", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
