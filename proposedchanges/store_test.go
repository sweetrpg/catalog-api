package proposedchanges

import (
	"context"
	"os"
	"testing"

	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/mongodb.go/constants"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func newTestProposal(recordID string) *ProposedChange {
	return &ProposedChange{
		RecordType: "volume",
		RecordID:   recordID,
		Diff: map[string]FieldChange{
			"title": {Old: "Old Title", New: "New Title", Status: StatusPending},
		},
		SubmittedBy: "auth0|submitter",
	}
}

func TestAddAndGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	recordID := primitive.NewObjectID().Hex()

	p := newTestProposal(recordID)
	id, err := Add(ctx, p)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if id == "" {
		t.Fatal("Add() returned empty id")
	}

	got, err := Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got == nil {
		t.Fatal("Get() = nil, want the proposal just added")
	}
	if got.RecordType != "volume" || got.RecordID != recordID {
		t.Errorf("Get() = %+v, want RecordType=volume RecordID=%s", got, recordID)
	}
	if got.Status != StatusPending {
		t.Errorf("Status = %q, want %q", got.Status, StatusPending)
	}
	field, ok := got.Diff["title"]
	if !ok {
		t.Fatal("Diff[\"title\"] missing after round trip")
	}
	if field.Old != "Old Title" || field.New != "New Title" {
		t.Errorf("Diff[\"title\"] = %+v, want Old=Old Title New=New Title", field)
	}
}

func TestListPendingReturnsOnlyPendingForRecord(t *testing.T) {
	ctx := context.Background()
	recordID := primitive.NewObjectID().Hex()
	otherRecordID := primitive.NewObjectID().Hex()

	pending, err := Add(ctx, newTestProposal(recordID))
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	other := newTestProposal(recordID)
	otherID, err := Add(ctx, other)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	fetched, err := Get(ctx, otherID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	fetched.Diff["title"] = FieldChange{Old: "Old Title", New: "New Title", Status: StatusAccepted}
	fetched.DeriveStatus()
	if err := Update(ctx, fetched); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	unrelated, err := Add(ctx, newTestProposal(otherRecordID))
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	_ = unrelated

	results, err := ListPending(ctx, "volume", recordID)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("ListPending() returned %d results, want 1", len(results))
	}
	if results[0].ID.Hex() != pending {
		t.Errorf("ListPending()[0].ID = %s, want %s", results[0].ID.Hex(), pending)
	}
}

func TestDeriveStatus(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]FieldChange
		want   string
	}{
		{"all pending", map[string]FieldChange{"a": {Status: StatusPending}}, StatusPending},
		{"all accepted", map[string]FieldChange{"a": {Status: StatusAccepted}, "b": {Status: StatusAccepted}}, StatusAccepted},
		{"all rejected", map[string]FieldChange{"a": {Status: StatusRejected}}, StatusRejected},
		{"mixed decided", map[string]FieldChange{"a": {Status: StatusAccepted}, "b": {Status: StatusRejected}}, StatusPartiallyAccepted},
		{"one still pending", map[string]FieldChange{"a": {Status: StatusAccepted}, "b": {Status: StatusPending}}, StatusPending},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &ProposedChange{Diff: tc.fields}
			p.DeriveStatus()
			if p.Status != tc.want {
				t.Errorf("DeriveStatus() = %q, want %q", p.Status, tc.want)
			}
		})
	}
}
