package data

import (
	"context"
	"fmt"

	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/database"
	"github.com/sweetrpg/catalog-api/logging"
	"github.com/sweetrpg/catalog-api/models"
	"github.com/sweetrpg/catalog-api/util"
	"github.com/sweetrpg/catalog-api/vo"
	options "go.jtlabs.io/query"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func GetPublisher(c context.Context, id string) (*vo.PublisherVO, error) {
	_, span := otel.Tracer("publisher").Start(c, "db-get-publisher", oteltrace.WithAttributes(attribute.String("id", id)))
	model, err := database.Get[models.Publisher]("publishers", id)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Publisher: %v", err))
		return nil, err
	}

	if model == nil {
		logging.Logger.Info(fmt.Sprintf("Publisher not found for ID: %s", id))
		return nil, nil
	}

	return &vo.PublisherVO{
		ID:         model.ID,
		Name:       model.Name,
		Address:    model.Address,
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

func GetPublishers(c context.Context, filter bson.D, options options.Options) ([]*vo.PublisherVO, error) {
	span := util.BuildSpanWithOptions(c, "publishers", "db-get-publishers", options)
	models, err := database.Query[models.Publisher]("publishers", filter, "_id", options.Page[constants.PageStartOption], options.Page[constants.PageLimitOption])
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Publishers: %v", err))
		return nil, err
	}

	modelCount := len(models)
	if modelCount == 0 {
		// short-circuit if there's nothing to do
		return make([]*vo.PublisherVO, 0), nil
	}

	var vos []*vo.PublisherVO
	for _, model := range models {
		vo, err := GetPublisher(c, model.ID)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No Publisher found from item in array for ID: %s", model.ID))
			continue
		}
		vos = append(vos, vo)
	}

	return vos, err
}
