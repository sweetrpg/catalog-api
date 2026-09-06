package server

import (
	"net/http"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-data.go/data"
)

const (
	volumeTagsLimitDefault = 20
	volumeTagsLimitMax     = 100
)

// Get the volume tag cloud - a bare JSON array of tag names ranked by usage, not a JSON:API
// resource. Unlike every other route here there is no single entity being returned, and the
// landing page consumes it directly, so the plain-array shape is the whole response (mirroring
// the plain-JSON choice made for `/stats`).
//
//	@Summary		Get volume tags
//	@Description	Returns the distinct tag names on live volumes, most-used first, capped by limit. The tag cloud drops its low-frequency tail, so callers pass a limit to control how many tags come back.
//	@Tags			volumes
//	@Produce		json
//	@Param			limit	query		int	false	"Maximum number of tags to return (default 20, max 100)"
//	@Success		200		{array}		string
//	@Failure		500		{object}	interface{}
//	@Router			/volumes/tags [get]
func getVolumeTags(c *gin.Context) {
	limit := volumeTagsLimitDefault
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		if parsed > volumeTagsLimitMax {
			parsed = volumeTagsLimitMax
		}
		limit = parsed
	}

	tags, err := data.GetVolumeTags(c.Request.Context(), limit)
	if err != nil {
		sentry.CaptureException(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}
