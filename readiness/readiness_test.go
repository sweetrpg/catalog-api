package readiness

import "testing"

func TestSetCacheReadyRoundTrips(t *testing.T) {
	SetCacheReady(true)
	if !CacheReady() {
		t.Fatal("CacheReady() = false after SetCacheReady(true), want true")
	}

	SetCacheReady(false)
	if CacheReady() {
		t.Fatal("CacheReady() = true after SetCacheReady(false), want false")
	}
}
