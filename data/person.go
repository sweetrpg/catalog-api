package data

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/models"
	"github.com/sweetrpg/catalog-api/vo"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func GetPerson(c context.Context, id string) (*vo.PersonVO, error) {
	_, span := otel.Tracer("person").Start(c, "db-get-person", oteltrace.WithAttributes(attribute.String("id", id)))
	model, err := database.Get[models.Person]("persons", id)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Person: %v", err))
		return nil, err
	}

	if model == nil {
		logging.Logger.Info(fmt.Sprintf("Person not found for ID: %s", id))
		return nil, nil
	}

	return &vo.PersonVO{
		ID:         model.ID,
		Name:       model.Name,
		Notes:      model.Notes,
		Properties: vo.FromPropertyModels(model.Properties),
		Tags:       vo.FromTagModels(model.Tags),
		CreatedAt:  model.CreatedAt,
		CreatedBy:  model.CreatedBy,
		UpdatedAt:  model.UpdatedAt,
		UpdatedBy:  model.UpdatedBy,
		DeletedAt:  model.DeletedAt,
		DeletedBy:  model.DeletedBy,
	}, nil
}

func GetPersons(c context.Context, start int, limit int) ([]*vo.PersonVO, error) {
	_, span := otel.Tracer("person").Start(c, "db-get-persons",
		oteltrace.WithAttributes(attribute.String("start", strconv.Itoa(start)),
			attribute.String("limit", strconv.Itoa(limit))))
	models, err := database.Query[models.Person]("persons", bson.D{}, "_id", start, limit)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Persons: %v", err))
		return nil, err
	}

	modelCount := len(models)
	if modelCount == 0 {
		// short-circuit if there's nothing to do
		return make([]*vo.PersonVO, 0), nil
	}

	var vos []*vo.PersonVO
	for _, model := range models {
		vo, err := GetPerson(c, model.ID)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No Person found from item in array for ID: %s", model.ID))
			continue
		}
		vos = append(vos, vo)
	}

	return vos, err
}
