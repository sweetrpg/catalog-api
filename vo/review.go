package vo

import (
	"time"
)

type ReviewVO struct {
	ID        string     `json:"id" jsonapi:"primary,review"`
	Title     string     `json:"title" jsonapi:"attr,title"`
	Body      string     `json:"body" jsonapi:"attr,body"`
	Language  string     `json:"language" jsonapi:"attr,language"`
	Tags      []TagVO    `json:"tags" jsonapi:"attr,tags"`
	Volume    *VolumeVO  `json:"volume" jsonapi:"relation,volume"`
	Notes     string     `json:"notes" jsonapi:"attr,notes"`
	CreatedAt time.Time  `json:"created_at" jsonapi:"attr,created_at"`
	CreatedBy string     `json:"created_by" jsonapi:"attr,created_by"`
	UpdatedAt time.Time  `json:"updated_at" jsonapi:"attr,updated_at"`
	UpdatedBy string     `json:"updated_by" jsonapi:"attr,updated_by"`
	DeletedAt *time.Time `json:"deleted_at" jsonapi:"attr,deleted_at,omitempty"`
	DeletedBy *string    `json:"deleted_by" jsonapi:"attr,deleted_by,omitempty"`
}
