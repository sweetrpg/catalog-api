package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	apiv "github.com/sweetrpg/api-core.go/vo"
	"github.com/sweetrpg/catalog-api/authz"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/catalog-objects.go/vo"
	"github.com/sweetrpg/common.go/logging"
	modelcore "github.com/sweetrpg/model-core.go/vo"
)

// createVolumeRequest is the POST /volumes body. It mirrors patchVolumeRequest's field set and
// wire names, but title is required and every field is a value, not a presence-pointer - a
// create has no "field omitted vs field set empty" distinction to preserve.
type createVolumeRequest struct {
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	Notes          string            `json:"notes"`
	Format         string            `json:"format"`
	Tags           []string          `json:"tags"`
	Properties     []propertyRequest `json:"properties"`
	PublisherIDs   []string          `json:"publisherIds"`
	StudioIDs      []string          `json:"studioIds"`
	SystemIDs      []string          `json:"systemIds"`
	CoverAssetId   string            `json:"coverAssetId"`
	SampleAssetIds []string          `json:"sampleAssetIds"`
}

// Create a volume.
//
//	@Summary		Create a volume
//	@Description	Creates a new volume. Like POST /publishers, a brand-new record is live immediately for every write role - there is no prior version for a submission to be reviewed against. Title is required.
//	@Tags			volumes
//	@Accept			json
//	@Produce		json
//	@Param			request	body		createVolumeRequest	true	"Volume to create"
//	@Success		201		{object}	interface{}
//	@Failure		400		{object}	apiv.ErrorVO
//	@Failure		500		{object}	apiv.ErrorVO
//	@Router			/volumes [post]
func createVolume(c *gin.Context, store persistence.CacheStore) {
	var req createVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: err.Error()})
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{Error: "invalid_request", Message: "title is required"})
		return
	}
	if len(req.SampleAssetIds) > maxSampleAssetIds {
		c.JSON(http.StatusBadRequest, apiv.ErrorVO{
			Error:   "sample_limit_exceeded",
			Message: fmt.Sprintf("a volume may have at most %d samples", maxSampleAssetIds),
		})
		return
	}

	subject := authz.Subject(c)
	newVolume := vo.VolumeVO{
		Title:          req.Title,
		Description:    req.Description,
		Notes:          req.Notes,
		Format:         req.Format,
		CoverAssetId:   req.CoverAssetId,
		SampleAssetIds: req.SampleAssetIds,
		Tags:           createVolumeTags(req.Tags),
		Properties:     createVolumeProperties(req.Properties),
	}
	for _, id := range req.PublisherIDs {
		newVolume.Publishers = append(newVolume.Publishers, &vo.PublisherVO{ID: id})
	}
	for _, id := range req.StudioIDs {
		newVolume.Studios = append(newVolume.Studios, &vo.StudioVO{ID: id})
	}
	for _, id := range req.SystemIDs {
		newVolume.Systems = append(newVolume.Systems, &vo.SystemVO{ID: id})
	}
	newVolume.CreatedBy = subject
	newVolume.UpdatedBy = subject

	id, err := data.AddVolume(c.Request.Context(), &newVolume)
	if err != nil {
		logging.Logger.Error("createVolume: add failed", "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "create_failed", Message: err.Error()})
		return
	}
	if id == nil {
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "create_failed", Message: "no id returned"})
		return
	}
	invalidateCachedPaths(store, "/volumes", "/volumes/"+*id, "/publishers", "/studios", "/systems")

	result, err := data.GetVolume(c.Request.Context(), *id)
	if err != nil {
		logging.Logger.Error("createVolume: post-create query failed", "id", *id, "error", err)
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, apiv.ErrorVO{Error: "query_failed", Message: err.Error()})
		return
	}

	logging.Logger.Info("createVolume: created", "id", *id, "createdBy", subject)
	c.Writer.Header().Set("Content-type", jsonapi.MediaType)
	c.Writer.WriteHeader(http.StatusCreated)
	if err := jsonapi.MarshalPayload(c.Writer, result); err != nil {
		sentry.CaptureException(err)
	}
}

func createVolumeTags(names []string) []modelcore.TagVO {
	if len(names) == 0 {
		return nil
	}
	out := make([]modelcore.TagVO, len(names))
	for i, name := range names {
		out[i] = modelcore.TagVO{Name: name}
	}
	return out
}

func createVolumeProperties(props []propertyRequest) []modelcore.PropertyVO {
	if len(props) == 0 {
		return nil
	}
	out := make([]modelcore.PropertyVO, len(props))
	for i, p := range props {
		out[i] = modelcore.PropertyVO{Name: p.Name, Kind: p.Kind, Value: p.Value}
	}
	return out
}
