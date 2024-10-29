package models

import (
	"time"
)

// Review model.
// This model represents a review on an RPG resource.
type Review struct {
	ID        string     `bson:"_id" json:"id" jsonapi:"primary,review"`
	Title     string     `json:"title" jsonapi:"attr,title"`
	Body      string     `bson:"body" json:"body" jsonapi:"attr,body"`
	Language  string     `json:"language" jsonapi:"attr,language"`
	Tags      []Tag      `json:"tags" jsonapi:"attr,tags"`
	VolumeId  string     `json:"volume_id" jsonapi:"relation,volume"`
	Notes     string     `json:"notes" jsonapi:"attr,notes"`
	CreatedAt time.Time  `bson:"created_at" json:"created_at" jsonapi:"attr,created_at"`
	CreatedBy string     `bson:"created_by" json:"created_by" jsonapi:"attr,created_by"`
	UpdatedAt time.Time  `bson:"updated_at" json:"updated_at" jsonapi:"attr,updated_at"`
	UpdatedBy string     `bson:"updated_by" json:"updated_by" jsonapi:"attr,updated_by"`
	DeletedAt *time.Time `bson:"deleted_at,omitempty" json:"deleted_at" jsonapi:"attr,deleted_at,omitempty"`
	DeletedBy *string    `bson:"deleted_by,omitempty" json:"deleted_by" jsonapi:"attr,deleted_by,omitempty"`
}
