package vo

import (
	"time"
)

// Volume value object.
// This value object is a serializable representation of the Volume model.
type VolumeVO struct {
	ID          string        `json:"id" jsonapi:"primary,volume"`
	Title       string        `json:"title" jsonapi:"attr,title"`
	Description string        `json:"description" jsonapi:"attr,description"`
	Notes       string        `json:"notes" jsonapi:"attr,notes"`
	Properties  []PropertyVO  `json:"properties" jsonapi:"attr,properties"`
	Tags        []TagVO       `json:"tags" jsonapi:"attr,tags"`
	Systems     []SystemVO    `json:"systems" jsonapi:"relation,system"`
	Publishers  []PublisherVO `json:"publishers" jsonapi:"relation,publisher"`
	Studios     []StudioVO    `json:"studios" jsonapi:"relation,studio"`
	Licenses    []LicenseVO   `json:"licenses" jsonapi:"relation,license"`
	CreatedAt   time.Time     `json:"created_at" jsonapi:"attr,created_at"`
	CreatedBy   string        `json:"created_by" jsonapi:"attr,created_by"`
	UpdatedAt   time.Time     `json:"updated_at" jsonapi:"attr,updated_at"`
	UpdatedBy   string        `json:"updated_by" jsonapi:"attr,updated_by"`
	DeletedAt   *time.Time    `json:"deleted_at" jsonapi:"attr,deleted_at,omitempty"`
	DeletedBy   *string       `json:"deleted_by" jsonapi:"attr,deleted_by,omitempty"`
}
