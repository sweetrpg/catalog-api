package util

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/catalog-api/constants"
)

type ListQueryParams struct {
	Start int
	Limit int
}

func GetListQueryParams(c *gin.Context) ListQueryParams {
	defaultLimit := float64(GetEnvInt(constants.QUERY_HARD_LIMIT, constants.QueryHardLimit))

	start, _ := strconv.Atoi(c.Query("start"))
	start = int(math.Max(0.0, float64(start)))

	limit, _ := strconv.Atoi(c.Query("limit"))
	limit = int(math.Min(defaultLimit, math.Max(1.0, float64(limit))))

	return ListQueryParams{
		Start: start,
		Limit: limit,
	}
}
