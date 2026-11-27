package server

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	apiutil "github.com/sweetrpg/api-core.go/util"
)

// listFields names the fields a client may use on one list route. Anything not listed is
// refused, so a new record field stays unqueryable until someone adds it here.
type listFields struct {
	filter []string
	sort   []string
}

// q is catalog-data's multi-field search term; only the types that expand it accept it.
var (
	volumeFields       = listFields{filter: []string{"q", "title", "tags.name", "tags.value"}, sort: []string{"title"}}
	namedFields        = listFields{filter: []string{"q", "name"}, sort: []string{"name"}}
	licenseFields      = listFields{filter: []string{"q", "title"}, sort: []string{"title"}}
	contributionFields = listFields{filter: []string{"volume_id"}}
	reviewFields       = listFields{}

	// Nested routes set their own filter, so they take none from the client. The sort applies
	// to the records the route queries.
	nestedVolumeFields = listFields{sort: volumeFields.sort}
	nestedFixedFields  = listFields{}
)

var (
	errFilterField = errors.New("unsupported filter field")
	errSortField   = errors.New("unsupported sort field")
	errProjection  = errors.New("the fields parameter is not supported")
)

func (f listFields) check(params apiutil.QueryParams) error {
	for _, filter := range params.Filter {
		if !slices.Contains(f.filter, filter.Field) {
			return errFilterField
		}
	}
	for _, s := range params.Sort {
		// One leading "-" means descending, matching catalog-data's normalizeSort.
		if !slices.Contains(f.sort, strings.TrimPrefix(s.Field, "-")) {
			return errSortField
		}
	}
	if len(params.Projection) > 0 {
		return errProjection
	}
	return nil
}

// listParams parses the query string of a list route and enforces allowed. On failure it has
// already answered 400 and returns false. It rejects rather than ignores, because dropping a
// filter would silently return a wider result than the caller asked for.
func listParams(c *gin.Context, allowed listFields) (apiutil.QueryParams, bool) {
	params, err := apiutil.GetQueryParams(c.Request.URL.RawQuery)
	if err == nil {
		err = allowed.check(params)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return apiutil.QueryParams{}, false
	}
	return params, true
}
