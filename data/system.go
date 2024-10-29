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

func GetSystem(c context.Context, id string) (*vo.SystemVO, error) {
	_, span := otel.Tracer("system").Start(c, "db-get-system", oteltrace.WithAttributes(attribute.String("id", id)))
	model, err := database.Get[models.System]("systems", id)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for System: %v", err))
		return nil, err
	}

	if model == nil {
		logging.Logger.Info(fmt.Sprintf("System not found for ID: %s", id))
		return nil, nil
	}

	return &vo.SystemVO{
		ID:         model.ID,
		GameSystem: model.GameSystem,
		Edition:    model.Edition,
		Notes:      model.Notes,
		Tags:       vo.FromTagModels(model.Tags),
		CreatedAt:  model.CreatedAt,
		CreatedBy:  model.CreatedBy,
		UpdatedAt:  model.UpdatedAt,
		UpdatedBy:  model.UpdatedBy,
		DeletedAt:  model.DeletedAt,
		DeletedBy:  model.DeletedBy,
	}, nil
}

func GetSystems(c context.Context, start int, limit int) ([]*vo.SystemVO, error) {
	_, span := otel.Tracer("system").Start(c, "db-get-systems",
		oteltrace.WithAttributes(attribute.String("start", strconv.Itoa(start)),
			attribute.String("limit", strconv.Itoa(limit))))
	models, err := database.Query[models.System]("systems", bson.D{}, "_id", start, limit)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Systems: %v", err))
		return nil, err
	}

	modelCount := len(models)
	if modelCount == 0 {
		// short-circuit if there's nothing to do
		return make([]*vo.SystemVO, 0), nil
	}

	var vos []*vo.SystemVO
	for _, model := range models {
		vo, err := GetSystem(c, model.ID)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No System found from item in array for ID: %s", model.ID))
			continue
		}
		vos = append(vos, vo)
	}

	return vos, err
}
