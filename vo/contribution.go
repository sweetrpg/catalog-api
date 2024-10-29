package vo

import (
	"time"
)

type ContributionVO struct {
	ID        string     `json:"id" jsonapi:"primary,contribution"`
	Person    *PersonVO  `json:"person,omitempty" jsonapi:"relation,person,omitempty"`
	Volume    *VolumeVO  `json:"volume,omitempty" jsonapi:"relation,volume,omitempty"`
	Roles     []string   `jsonapi:"attr,roles"`
	Notes     string     `json:"notes" jsonapi:"attr,notes"`
	CreatedAt time.Time  `json:"created_at" jsonapi:"attr,created_at"`
	CreatedBy string     `json:"created_by" jsonapi:"attr,created_by"`
	UpdatedAt time.Time  `json:"updated_at" jsonapi:"attr,updated_at"`
	UpdatedBy string     `json:"updated_by" jsonapi:"attr,updated_by"`
	DeletedAt *time.Time `json:"deleted_at" jsonapi:"attr,deleted_at,omitempty"`
	DeletedBy *string    `json:"deleted_by" jsonapi:"attr,deleted_by,omitempty"`
}
