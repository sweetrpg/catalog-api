package models

import (
	"time"
)

// Person model.
// This model represents a person involved in the production of an RPG resource.
type Person struct {
	ID         string     `bson:"_id" json:"id" jsonapi:"primary,person"`
	Name       string     `json:"name" jsonapi:"attr,name"`
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
