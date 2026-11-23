package server

import "testing"

func TestMergeTitleHintsOverlaysHintsOnExistingTitles(t *testing.T) {
	existing := map[string]string{"a": "Old A", "b": "B"}
	got := mergeTitleHints(existing, map[string]string{"a": "New A", "c": "C"})
	if len(got) != 3 || got["a"] != "New A" || got["b"] != "B" || got["c"] != "C" {
		t.Errorf("merged = %v", got)
	}
	if existing["a"] != "Old A" {
		t.Errorf("existing map was modified: %v", existing)
	}
}

func TestMergeTitleHintsHandlesNilMaps(t *testing.T) {
	if got := mergeTitleHints(nil, nil); got == nil || len(got) != 0 {
		t.Errorf("merged = %v, want empty non-nil map", got)
	}
	if got := mergeTitleHints(nil, map[string]string{"x": "X"}); got["x"] != "X" {
		t.Errorf("merged = %v", got)
	}
}
