package events

import (
	"encoding/json"
	"testing"
)

func envelopeBody(t *testing.T, action, entityID, title string) []byte {
	t.Helper()
	data, _ := json.Marshal(map[string]string{"title": title})
	b, err := json.Marshal(Envelope{
		EventID: "e1", Source: "game-systems-api", EntityType: "system",
		EntityID: entityID, Action: action, Data: data,
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return b
}

func TestDecodeSystemUpdate(t *testing.T) {
	id, title, ok := decodeSystemUpdate(envelopeBody(t, "updated", "sys1", "Numenera"))
	if !ok || id != "sys1" || title != "Numenera" {
		t.Fatalf("got (%q, %q, %v), want (sys1, Numenera, true)", id, title, ok)
	}
}

func TestDecodeSystemUpdateIgnoresNonUpdated(t *testing.T) {
	for _, action := range []string{"created", "deleted"} {
		if _, _, ok := decodeSystemUpdate(envelopeBody(t, action, "sys1", "X")); ok {
			t.Errorf("action %q should not be handled", action)
		}
	}
}

func TestDecodeSystemUpdateRejectsEmptyEntityID(t *testing.T) {
	if _, _, ok := decodeSystemUpdate(envelopeBody(t, "updated", "", "X")); ok {
		t.Error("empty entity_id should not be handled")
	}
}

func TestDecodeSystemUpdateRejectsGarbage(t *testing.T) {
	if _, _, ok := decodeSystemUpdate([]byte("not json")); ok {
		t.Error("undecodable body should not be handled (dropped, not poisoned)")
	}
}
