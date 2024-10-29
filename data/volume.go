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

func GetVolume(c context.Context, id string) (*vo.VolumeVO, error) {
	_, span := otel.Tracer("volume").Start(c, "db-get-volume", oteltrace.WithAttributes(attribute.String("id", id)))
	model, err := database.Get[models.Volume]("volumes", id)
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Volume: %v", err))
		return nil, err
	}

	if model == nil {
		logging.Logger.Info(fmt.Sprintf("Volume not found for ID: %s", id))
		return nil, nil
	}

	systemVOs := util.Map[string, vo.SystemVO](model.SystemIds, func(id string) *vo.SystemVO {
		vo, err := GetSystem(c, id)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No System found from Volume for ID %s: %s", id, err.Error()))
		}
		return vo
	})
	publisherVOs := util.Map[string, vo.PublisherVO](model.PublisherIds, func(id string) *vo.PublisherVO {
		vo, err := GetPublisher(c, id)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No Publisher found from Volume for ID %s: %s", id, err.Error()))
		}
		return vo
	})
	studioVOs := util.Map[string, vo.StudioVO](model.StudioIds, func(id string) *vo.StudioVO {
		vo, err := GetStudio(c, id)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No Studio found from Volume for ID %s: %s", id, err.Error()))
		}
		return vo
	})
	licenseVOs := util.Map[string, vo.LicenseVO](model.LicenseIds, func(id string) *vo.LicenseVO {
		vo, err := GetLicense(c, id)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No License found from Volume for ID %s: %s", id, err.Error()))
		}
		return vo
	})

	return &vo.VolumeVO{
		ID:          model.ID,
		Title:       model.Title,
		Description: model.Description,
		Notes:       model.Notes,
		Properties:  vo.FromPropertyModels(model.Properties),
		Tags:        vo.FromTagModels(model.Tags),
		Systems:     systemVOs,
		Publishers:  publisherVOs,
		Studios:     studioVOs,
		Licenses:    licenseVOs,
		CreatedAt:   model.CreatedAt,
		CreatedBy:   model.CreatedBy,
		UpdatedAt:   model.UpdatedAt,
		UpdatedBy:   model.UpdatedBy,
		DeletedAt:   model.DeletedAt,
		DeletedBy:   model.DeletedBy,
	}, nil
}

func GetVolumes(c context.Context, filter bson.D, options options.Options) ([]*vo.VolumeVO, error) {
	span := util.BuildSpanWithOptions(c, "volumes", "db-get-volumes", options)
	models, err := database.Query[models.Volume]("volumes", filter, "_id", options.Page[constants.PageStartOption], options.Page[constants.PageLimitOption])
	span.End()
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while querying database for Volumes: %v", err))
		return nil, err
	}

	modelCount := len(models)
	if modelCount == 0 {
		// short-circuit if there's nothing to do
		return make([]*vo.VolumeVO, 0), nil
	}

	var vos []*vo.VolumeVO
	for _, model := range models {
		vo, err := GetVolume(c, model.ID)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("No Volume found from item in array for ID: %s", model.ID))
			continue
		}
		vos = append(vos, vo)
	}

	return vos, err
}
