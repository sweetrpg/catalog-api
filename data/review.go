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

func GetReview(c context.Context, id string) (*vo.ReviewVO, error) {
	_, span := otel.Tracer("review").Start(c, "db-get-review", oteltrace.WithAttributes(attribute.String("id", id)))
	model, err := database.Get[models.Review]("reviews", id)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Review: %v", err))
		return nil, err
	}

	if model == nil {
		logging.Logger.Info(fmt.Sprintf("Review not found for ID: %s", id))
		return nil, nil
	}

	volumeVO, err := GetVolume(c, model.VolumeId)
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("No Volume found from Review for ID %s: %s", model.VolumeId, err.Error()))
	}

	return &vo.ReviewVO{
		ID:        model.ID,
		Title:     model.Title,
		Body:      model.Body,
		Language:  model.Language,
		Tags:      vo.FromTagModels(model.Tags),
		Volume:    volumeVO,
		CreatedAt: model.CreatedAt,
		CreatedBy: model.CreatedBy,
		UpdatedAt: model.UpdatedAt,
		UpdatedBy: model.UpdatedBy,
		DeletedAt: model.DeletedAt,
		DeletedBy: model.DeletedBy,
	}, nil
}

func GetReviews(c context.Context, filter bson.D, options options.Options) ([]*vo.ReviewVO, error) {
	span := util.BuildSpanWithOptions(c, "reviews", "db-get-reviews", options)
	models, err := database.Query[models.Review]("reviews", filter, "_id", options.Page[constants.PageStartOption], options.Page[constants.PageLimitOption])
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Reviews: %v", err))
		return nil, err
	}

	modelCount := len(models)
	if modelCount == 0 {
		// short-circuit if there's nothing to do
		return make([]*vo.ReviewVO, 0), nil
	}

	var vos []*vo.ReviewVO
	for _, model := range models {
		vo, err := GetReview(c, model.ID)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No Review found from item in array for ID: %s", model.ID))
			continue
		}
		vos = append(vos, vo)
	}

	return vos, err
}
