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

func GetLicense(c context.Context, id string) (*vo.LicenseVO, error) {
	_, span := otel.Tracer("license").Start(c, "db-get-license", oteltrace.WithAttributes(attribute.String("id", id)))
	model, err := database.Get[models.License]("licenses", id)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for License: %v", err))
		return nil, err
	}

	if model == nil {
		logging.Logger.Info(fmt.Sprintf("License not found for ID: %s", id))
		return nil, nil
	}

	return &vo.LicenseVO{
		ID:           model.ID,
		Title:        model.Title,
		ShortTitle:   model.ShortTitle,
		Version:      model.Version,
		Deed:         model.Deed,
		LegalCode:    model.LegalCode,
		URL:          model.URL,
		Status:       model.Status,
		Availability: model.Availability,
		Notes:        model.Notes,
		Properties:   vo.FromPropertyModels(model.Properties),
		Tags:         vo.FromTagModels(model.Tags),
		CreatedAt:    model.CreatedAt,
		CreatedBy:    model.CreatedBy,
		UpdatedAt:    model.UpdatedAt,
		UpdatedBy:    model.UpdatedBy,
		DeletedAt:    model.DeletedAt,
		DeletedBy:    model.DeletedBy,
	}, nil
}

func GetLicenses(c context.Context, filter bson.D, options options.Options) ([]*vo.LicenseVO, error) {
	span := util.BuildSpanWithOptions(c, "licenses", "db-get-licenses", options)
	models, err := database.Query[models.License]("licenses", filter, "_id", options.Page[constants.PageStartOption], options.Page[constants.PageLimitOption])
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Licenses: %v", err))
		return nil, err
	}

	modelCount := len(models)
	if modelCount == 0 {
		// short-circuit if there's nothing to do
		return make([]*vo.LicenseVO, 0), nil
	}

	var vos []*vo.LicenseVO
	for _, model := range models {
		vo, err := GetLicense(c, model.ID)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No License found from item in array for ID: %s", model.ID))
			continue
		}
		vos = append(vos, vo)
	}

	return vos, err
}
