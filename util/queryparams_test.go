package util

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/sweetrpg/catalog-api/constants"
)

func TestWithGoodValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "http://localhost:1234?start=0&limit=5", httptest.NewRecorder().Body)

	listParams := GetListQueryParams(c)
	assert.Equal(t, 0, listParams.Start)
	assert.Equal(t, 5, listParams.Limit)
}

func TestWithNoStart(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "http://localhost:1234?limit=5", httptest.NewRecorder().Body)

	listParams := GetListQueryParams(c)
	assert.Equal(t, 0, listParams.Start)
	assert.Equal(t, 5, listParams.Limit)
}

func TestWithLowStart(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "http://localhost:1234?start=-7&limit=5", httptest.NewRecorder().Body)

	listParams := GetListQueryParams(c)
	assert.Equal(t, 0, listParams.Start)
	assert.Equal(t, 5, listParams.Limit)
}

func TestWithNoLimit(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "http://localhost:1234?start=0", httptest.NewRecorder().Body)

	listParams := GetListQueryParams(c)
	assert.Equal(t, 0, listParams.Start)
	assert.Equal(t, 1, listParams.Limit)
}

func TestWithLowLimit(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "http://localhost:1234?start=0&limit=-1", httptest.NewRecorder().Body)

	listParams := GetListQueryParams(c)
	assert.Equal(t, 0, listParams.Start)
	assert.Equal(t, 1, listParams.Limit)
}

func TestWithHighLimit(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "http://localhost:1234?start=0&limit=54321", httptest.NewRecorder().Body)

	listParams := GetListQueryParams(c)
	assert.Equal(t, 0, listParams.Start)
	assert.Equal(t, constants.QueryHardLimit, listParams.Limit)
}
