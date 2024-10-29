package vo

import (
	"github.com/sweetrpg/catalog-api/models"
	"github.com/sweetrpg/catalog-api/util"
)

type PropertyVO struct {
	Name  string `json:"name" jsonapi:"attr,name"`
	Kind  string `json:"kind" jsonapi:"attr,kind"`
	Value string `json:"value" jsonapi:"attr,value"`
}

func FromPropertyModels(models []models.Property) []PropertyVO {
	return util.Map(models, FromPropertyModel)
}

func FromPropertyModel(model models.Property) *PropertyVO {
	return &PropertyVO{
		Name:  model.Name,
		Kind:  model.Kind,
		Value: model.Value,
	}
}
