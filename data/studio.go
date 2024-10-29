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

func GetStudio(c context.Context, id string) (*vo.StudioVO, error) {
	_, span := otel.Tracer("studio").Start(c, "db-get-studio", oteltrace.WithAttributes(attribute.String("id", id)))
	model, err := database.Get[models.Studio]("studios", id)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Studio: %v", err))
		return nil, err
	}

	if model == nil {
		logging.Logger.Info(fmt.Sprintf("Studio not found for ID: %s", id))
		return nil, nil
	}

	return &vo.StudioVO{
		ID:         model.ID,
		Name:       model.Name,
		Website:    model.Website,
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

func GetStudios(c context.Context, start int, limit int) ([]*vo.StudioVO, error) {
	_, span := otel.Tracer("studio").Start(c, "db-get-studios",
		oteltrace.WithAttributes(attribute.String("start", strconv.Itoa(start)),
			attribute.String("limit", strconv.Itoa(limit))))
	models, err := database.Query[models.Studio]("studios", bson.D{}, "_id", start, limit)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Studios: %v", err))
		return nil, err
	}

	modelCount := len(models)
	if modelCount == 0 {
		// short-circuit if there's nothing to do
		return make([]*vo.StudioVO, 0), nil
	}

	var vos []*vo.StudioVO
	for _, model := range models {
		vo, err := GetStudio(c, model.ID)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No Studio found from item in array for ID: %s", model.ID))
			continue
		}
		vos = append(vos, vo)
	}

	return vos, err
}
