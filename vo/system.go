package vo

import (
	"time"
)

type SystemVO struct {
	ID         string     `json:"id" jsonapi:"primary,system"`
	GameSystem string     `json:"game_system" jsonapi:"attr,game_system"`
	Edition    string     `json:"edition" jsonapi:"attr,edition"`
	Tags       []TagVO    `json:"tags" jsonapi:"attr,tags"`
	Notes      string     `json:"notes" jsonapi:"attr,notes"`
	CreatedAt  time.Time  `json:"created_at" jsonapi:"attr,created_at"`
	CreatedBy  string     `json:"created_by" jsonapi:"attr,created_by"`
	UpdatedAt  time.Time  `json:"updated_at" jsonapi:"attr,updated_at"`
	UpdatedBy  string     `json:"updated_by" jsonapi:"attr,updated_by"`
	DeletedAt  *time.Time `json:"deleted_at" jsonapi:"attr,deleted_at,omitempty"`
	DeletedBy  *string    `json:"deleted_by" jsonapi:"attr,deleted_by,omitempty"`
}
