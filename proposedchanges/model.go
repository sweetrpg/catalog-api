// Package proposedchanges stores field-level proposed edits to a record (a volume today, other
// record types later - see design.md's "generic proposed-change shape" decision) separately
// from the live record until an editor/admin reviews them. See platform's
// volume-proposed-changes spec (openspec/changes/volume-edit-with-approval-workflow in
// sweetrpg/platform).
package proposedchanges

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const CollectionName = "proposed_changes"

// Field-level and overall proposal statuses.
const (
	StatusPending           = "pending"
	StatusAccepted          = "accepted"
	StatusRejected          = "rejected"
	StatusPartiallyAccepted = "partially_accepted"
)

// FieldChange is one changed field's old (live-at-submission-time) and proposed value, plus its
// own review outcome once decided.
type FieldChange struct {
	Old    any    `bson:"old" json:"old"`
	New    any    `bson:"new" json:"new"`
	Status string `bson:"status" json:"status"`
}

// ProposedChange is a submitter's proposed edit to a live record, pending admin/editor review.
type ProposedChange struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	RecordType  string                 `bson:"record_type" json:"recordType"`
	RecordID    string                 `bson:"record_id" json:"recordId"`
	Diff        map[string]FieldChange `bson:"diff" json:"diff"`
	Status      string                 `bson:"status" json:"status"`
	SubmittedBy string                 `bson:"submitted_by" json:"submittedBy"`
	SubmittedAt time.Time              `bson:"submitted_at" json:"submittedAt"`
	ReviewedBy  string                 `bson:"reviewed_by,omitempty" json:"reviewedBy,omitempty"`
	ReviewedAt  *time.Time             `bson:"reviewed_at,omitempty" json:"reviewedAt,omitempty"`
	ReviewNote  string                 `bson:"review_note,omitempty" json:"reviewNote,omitempty"`
}

// DeriveStatus recomputes Status from each field's individual outcome: pending while any field
// is undecided, accepted/rejected if every decided field agrees, partially_accepted on a mix.
// Called after any review action so the top-level status never has to be set independently of
// the fields it's derived from.
func (p *ProposedChange) DeriveStatus() {
	accepted, rejected, pending := 0, 0, 0
	for _, f := range p.Diff {
		switch f.Status {
		case StatusAccepted:
			accepted++
		case StatusRejected:
			rejected++
		default:
			pending++
		}
	}

	switch {
	case pending > 0:
		p.Status = StatusPending
	case accepted > 0 && rejected > 0:
		p.Status = StatusPartiallyAccepted
	case accepted > 0:
		p.Status = StatusAccepted
	default:
		p.Status = StatusRejected
	}
}
