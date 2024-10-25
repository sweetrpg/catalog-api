package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type License struct {
	ID           primitive.ObjectID `bson:"_id"`
	Title        string
	ShortTitle   string `bson:"short_title"`
	Version      string
	Deed         string
	LegalCode    string `bson:"legal_code"`
	URL          string
	Status       string
	Availability string
	Notes        string
	//	Properties []Property `bson:"properties"`
	// Tags []Tag `bson:"tags"`
	// Volumes []string `bson:"volumes"`
	CreatedAt time.Time  `bson:"created_at"`
	CreatedBy string     `bson:"created_by"`
	UpdatedAt time.Time  `bson:"updated_at"`
	UpdatedBy string     `bson:"updated_by"`
	DeletedAt *time.Time `bson:"deleted_at"`
	DeletedBy *string    `bson:"deleted_by"`
}
