package events

import (
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
)

// PublishLicenseEvent publishes a license entity event.
func PublishLicenseEvent(pub *Publisher) func(c *gin.Context, id string, revision int, action string, data vo.LicenseVO) {
	return func(c *gin.Context, id string, revision int, action string, data vo.LicenseVO) {
		if pub == nil {
			return
		}
		ctx := c.Request.Context()
		switch action {
		case "created":
			pub.PublishEntityCreated(ctx, "license", id, revision, data)
		case "updated":
			pub.PublishEntityUpdated(ctx, "license", id, revision, data)
		case "deleted":
			pub.PublishEntityDeleted(ctx, "license", id)
		default:
			logging.Logger.Warn("PublishLicenseEvent: unknown action", "entity_id", id, "action", action)
		}
	}
}

// PublishPersonEvent publishes a person entity event.
func PublishPersonEvent(pub *Publisher) func(c *gin.Context, id string, revision int, action string, data vo.PersonVO) {
	return func(c *gin.Context, id string, revision int, action string, data vo.PersonVO) {
		if pub == nil {
			return
		}
		ctx := c.Request.Context()
		switch action {
		case "created":
			pub.PublishEntityCreated(ctx, "person", id, revision, data)
		case "updated":
			pub.PublishEntityUpdated(ctx, "person", id, revision, data)
		case "deleted":
			pub.PublishEntityDeleted(ctx, "person", id)
		default:
			logging.Logger.Warn("PublishPersonEvent: unknown action", "entity_id", id, "action", action)
		}
	}
}

// PublishPublisherEvent publishes a publisher entity event.
func PublishPublisherEvent(pub *Publisher) func(c *gin.Context, id string, revision int, action string, data vo.PublisherVO) {
	return func(c *gin.Context, id string, revision int, action string, data vo.PublisherVO) {
		if pub == nil {
			return
		}
		ctx := c.Request.Context()
		switch action {
		case "created":
			pub.PublishEntityCreated(ctx, "publisher", id, revision, data)
		case "updated":
			pub.PublishEntityUpdated(ctx, "publisher", id, revision, data)
		case "deleted":
			pub.PublishEntityDeleted(ctx, "publisher", id)
		default:
			logging.Logger.Warn("PublishPublisherEvent: unknown action", "entity_id", id, "action", action)
		}
	}
}

// PublishStudioEvent publishes a studio entity event.
func PublishStudioEvent(pub *Publisher) func(c *gin.Context, id string, revision int, action string, data vo.StudioVO) {
	return func(c *gin.Context, id string, revision int, action string, data vo.StudioVO) {
		if pub == nil {
			return
		}
		ctx := c.Request.Context()
		switch action {
		case "created":
			pub.PublishEntityCreated(ctx, "studio", id, revision, data)
		case "updated":
			pub.PublishEntityUpdated(ctx, "studio", id, revision, data)
		case "deleted":
			pub.PublishEntityDeleted(ctx, "studio", id)
		default:
			logging.Logger.Warn("PublishStudioEvent: unknown action", "entity_id", id, "action", action)
		}
	}
}
