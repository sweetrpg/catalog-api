package server

import (
	"testing"

	"github.com/sweetrpg/api-core.go/vo"
)

func TestNoteCacheStatusDoesNotFailHealth(t *testing.T) {
	for name, tc := range map[string]struct {
		ready    bool
		messages int
	}{
		"reachable":   {ready: true, messages: 0},
		"unreachable": {ready: false, messages: 1},
	} {
		t.Run(name, func(t *testing.T) {
			var resp vo.HealthResponseVO

			noteCacheStatus(&resp, tc.ready)

			if resp.Errors != 0 || len(resp.Messages) != tc.messages {
				t.Fatalf("errors = %d, messages = %v; want 0 errors, %d messages", resp.Errors, resp.Messages, tc.messages)
			}
		})
	}
}
