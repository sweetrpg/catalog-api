package vo

import (
	"net/url"
	"time"
)

// License value object.
// This value object is a serializable representation of the License model.
type LicenseVO struct {
	ID           string       `json:"id" jsonapi:"primary,license"`
	Title        string       `json:"title" jsonapi:"attr,title"`
	ShortTitle   string       `json:"short_title" jsonapi:"attr,short_title"`
	Version      string       `json:"version" jsonapi:"attr,version"`
	Deed         string       `json:"deed" jsonapi:"attr,deed"`
	LegalCode    string       `json:"legal_code" jsonapi:"attr,legal_code"`
	URL          url.URL      `json:"url" jsonapi:"attr,url"`
	Status       string       `json:"status" jsonapi:"attr,status"`
	Availability string       `json:"availability" jsonapi:"attr,availability"`
	Notes        string       `json:"notes" jsonapi:"attr,notes"`
	Properties   []PropertyVO `json:"properties" jsonapi:"attr,properties"`
	Tags         []TagVO      `json:"tags" jsonapi:"attr,tags"`
	CreatedAt    time.Time    `json:"created_at" jsonapi:"attr,created_at"`
	CreatedBy    string       `json:"created_by" jsonapi:"attr,created_by"`
	UpdatedAt    time.Time    `json:"updated_at" jsonapi:"attr,updated_at"`
	UpdatedBy    string       `json:"updated_by" jsonapi:"attr,updated_by"`
	DeletedAt    *time.Time   `json:"deleted_at" jsonapi:"attr,deleted_at,omitempty"`
	DeletedBy    *string      `json:"deleted_by" jsonapi:"attr,deleted_by,omitempty"`
}
