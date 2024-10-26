package models

import (
	"net/url"
	"time"
)

// Studio model.
// This model represents a studio that can produce an RPG resource.
type Studio struct {
	ID         string     `bson:"_id" json:"id" jsonapi:"primary,studio"`
	Name       string     `json:"name" jsonapi:"attr,name"`
	Website    url.URL    `bson:"website" json:"website" jsonapi:"attr,website"`
	Notes      string     `json:"notes" jsonapi:"attr,notes"`
	Properties []Property `json:"properties" jsonapi:"attr,properties"`
	Tags       []Tag      `json:"tags" jsonapi:"attr,tags"`
	CreatedAt  time.Time  `bson:"created_at" json:"created_at" jsonapi:"attr,created_at"`
	CreatedBy  string     `bson:"created_by" json:"created_by" jsonapi:"attr,created_by"`
	UpdatedAt  time.Time  `bson:"updated_at" json:"updated_at" jsonapi:"attr,updated_at"`
	UpdatedBy  string     `bson:"updated_by" json:"updated_by" jsonapi:"attr,updated_by"`
	DeletedAt  *time.Time `bson:"deleted_at,omitempty" json:"deleted_at" jsonapi:"attr,deleted_at,omitempty"`
	DeletedBy  *string    `bson:"deleted_by,omitempty" json:"deleted_by" jsonapi:"attr,deleted_by,omitempty"`
}
