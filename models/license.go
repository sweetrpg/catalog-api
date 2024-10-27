package models

import (
	"time"
)

// License model.
// This model represents the license information for any given RPG resource.
type License struct {
	ID           string     `bson:"_id" json:"id" jsonapi:"primary,license"`
	Title        string     `json:"title" jsonapi:"attr,title"`
	ShortTitle   string     `bson:"short_title" json:"short_title" jsonapi:"attr,short_title"`
	Version      string     `json:"version" jsonapi:"attr,version"`
	Deed         string     `json:"deed" jsonapi:"attr,deed"`
	LegalCode    string     `bson:"legal_code" json:"legal_code" jsonapi:"attr,legal_code"`
	URL          string     `json:"url" jsonapi:"attr,url"`
	Status       string     `json:"status" jsonapi:"attr,status"`
	Availability string     `json:"availability" jsonapi:"attr,availability"`
	Notes        string     `json:"notes" jsonapi:"attr,notes"`
	Properties   []Property `json:"properties" jsonapi:"attr,properties"`
	Tags         []Tag      `json:"tags" jsonapi:"attr,tags"`
	CreatedAt    time.Time  `bson:"created_at" json:"created_at" jsonapi:"attr,created_at"`
	CreatedBy    string     `bson:"created_by" json:"created_by" jsonapi:"attr,created_by"`
	UpdatedAt    time.Time  `bson:"updated_at" json:"updated_at" jsonapi:"attr,updated_at"`
	UpdatedBy    string     `bson:"updated_by" json:"updated_by" jsonapi:"attr,updated_by"`
	DeletedAt    *time.Time `bson:"deleted_at,omitempty" json:"deleted_at" jsonapi:"attr,deleted_at,omitempty"`
	DeletedBy    *string    `bson:"deleted_by,omitempty" json:"deleted_by" jsonapi:"attr,deleted_by,omitempty"`
}
