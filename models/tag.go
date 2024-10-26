package models

type Tag struct {
	Name  string `json:"name" jsonapi:"attr,name"`
	Value string `json:"value" jsonapi:"attr,value"`
}
