package server

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/sweetrpg/catalog-objects.go/vo"
)

func TestVolumeListCarriesAuditAndDeletionAttributes(t *testing.T) {
	deletedAt := time.Date(2026, 11, 20, 8, 30, 0, 123456789, time.UTC)
	deletedBy := "user-9"
	v := &vo.VolumeVO{ID: "v1", Title: "Dup", CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), CreatedBy: "u1",
		UpdatedAt: time.Date(2026, 6, 7, 8, 9, 10, 0, time.UTC), UpdatedBy: "u2", DeletedAt: &deletedAt, DeletedBy: &deletedBy}

	var buf bytes.Buffer
	if err := marshalListWithTotal(&buf, []*vo.VolumeVO{v}, 1); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Data []struct {
			Attributes map[string]any `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil || len(doc.Data) != 1 {
		t.Fatalf("decode: %v, body %s", err, buf.String())
	}
	attrs := doc.Data[0].Attributes
	if attrs["deleted_at"] != "2026-11-20T08:30:00Z" || attrs["deleted_by"] != "user-9" || attrs["created_at"] != "2026-01-02T03:04:05Z" {
		t.Errorf("audit attributes missing or malformed: %v", attrs)
	}
}
