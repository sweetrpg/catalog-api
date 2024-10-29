package vo

import (
	"net/url"
	"time"
)

type PublisherVO struct {
	ID         string       `json:"id" jsonapi:"primary,publisher"`
	Name       string       `json:"name" jsonapi:"attr,name"`
	Address    string       `json:"address" jsonapi:"attr,address"`
	Website    url.URL      `bson:"website" json:"website" jsonapi:"attr,website"`
	Notes      string       `json:"notes" jsonapi:"attr,notes"`
	Properties []PropertyVO `json:"properties" jsonapi:"attr,properties"`
	Tags       []TagVO      `json:"tags" jsonapi:"attr,tags"`
	CreatedAt  time.Time    `json:"created_at" jsonapi:"attr,created_at"`
	CreatedBy  string       `json:"created_by" jsonapi:"attr,created_by"`
	UpdatedAt  time.Time    `json:"updated_at" jsonapi:"attr,updated_at"`
	UpdatedBy  string       `json:"updated_by" jsonapi:"attr,updated_by"`
	DeletedAt  *time.Time   `json:"deleted_at" jsonapi:"attr,deleted_at,omitempty"`
	DeletedBy  *string      `json:"deleted_by" jsonapi:"attr,deleted_by,omitempty"`
}
