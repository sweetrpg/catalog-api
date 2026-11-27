package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/internal/events"
)

const listTestID = "0123456789abcdef01234567"

var listRoutes = []struct {
	path    string
	allowed listFields
}{
	{"/volumes", volumeFields},
	{"/volumes/" + listTestID + "/reviews", nestedFixedFields},
	{"/publishers", namedFields},
	{"/publishers/" + listTestID + "/volumes", nestedVolumeFields},
	{"/studios", namedFields},
	{"/studios/" + listTestID + "/volumes", nestedVolumeFields},
	{"/persons", namedFields},
	{"/persons/" + listTestID + "/volumes", nestedFixedFields},
	{"/licenses", licenseFields},
	{"/licenses/" + listTestID + "/volumes", nestedVolumeFields},
	{"/contributions", contributionFields},
	{"/reviews", reviewFields},
}

func newListRouter(t *testing.T) *gin.Engine {
	return newRelationshipTestRouter(t, func(r *gin.Engine, s persistence.CacheStore, ttls cachettl.Config, a *authz.Client, p *events.Publisher) {
		setupVolumeHandlers(r, s, ttls, a, nil, nil, p)
		setupPublisherHandlers(r, s, ttls, a, p)
		setupStudioHandlers(r, s, ttls, a, p)
		setupPersonHandlers(r, s, ttls, a, p)
		setupLicenseHandlers(r, s, ttls, a, p)
		setupContributionHandlers(r, s, ttls)
		setupReviewHandlers(r, s, ttls)
	})
}

// Rejection happens before any query, so every case must answer 400 on every list route.
func TestListRoutesRefuseUnlistedParameters(t *testing.T) {
	cases := map[string]struct{ query, want string }{
		"unlisted filter":                {"filter[notes]=x", errFilterField.Error()},
		"unlisted filter with eq":        {"filter[notes][eq]=x", errFilterField.Error()},
		"unlisted filter with contains":  {"filter[notes][contains]=x", errFilterField.Error()},
		"unlisted filter with in":        {"filter[notes][in]=x,y", errFilterField.Error()},
		"unlisted filter, encoded":       {"filter%5Bnotes%5D=x", errFilterField.Error()},
		"state filter":                   {"filter[state]=submitted", errFilterField.Error()},
		"case variant":                   {"filter[Name]=x", errFilterField.Error()},
		"subfield of a listed field":     {"filter[name.notes]=x", errFilterField.Error()},
		"allowed beside unlisted filter": {"filter[q]=a&filter[notes]=x", errFilterField.Error()},
		"unlisted sort":                  {"sort=notes", errSortField.Error()},
		"unlisted descending sort":       {"sort=-notes", errSortField.Error()},
		"unlisted second sort":           {"sort=name,notes", errSortField.Error()},
		"doubled sort prefix":            {"sort=--name", errSortField.Error()},
		"projection":                     {"fields=name", errProjection.Error()},
	}

	r := newListRouter(t)
	for _, route := range listRoutes {
		for name, tc := range cases {
			t.Run(route.path+" "+name, func(t *testing.T) {
				rec := doGet(t, r, route.path+"?"+tc.query)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400, body: %s", rec.Code, rec.Body.String())
				}
				if !strings.Contains(rec.Body.String(), tc.want) {
					t.Fatalf("body = %s, want %q", rec.Body.String(), tc.want)
				}
			})
		}
	}
}

func TestListFieldSetsAcceptExactlyTheirFields(t *testing.T) {
	want := map[string]struct{ filters, sorts string }{
		"/volumes":       {"q title tags.value", "title"},
		"/publishers":    {"q name", "name"},
		"/studios":       {"q name", "name"},
		"/persons":       {"q name", "name"},
		"/licenses":      {"q title", "title"},
		"/contributions": {"volume_id", ""},
		"/reviews":       {"", ""},
	}
	parse := func(allowed listFields, query string) bool {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/x?"+query+"&page[start]=0&page[limit]=5", nil)
		_, ok := listParams(c, allowed)
		return ok
	}
	for _, route := range listRoutes {
		w, top := want[route.path]
		if !top {
			if len(route.allowed.filter) != 0 {
				t.Errorf("%s: nested route accepts client filters %v", route.path, route.allowed.filter)
			}
			continue
		}
		if got := strings.Join(route.allowed.filter, " "); got != w.filters {
			t.Errorf("%s: filters = %q, want %q", route.path, got, w.filters)
		}
		if got := strings.Join(route.allowed.sort, " "); got != w.sorts {
			t.Errorf("%s: sorts = %q, want %q", route.path, got, w.sorts)
		}
		for _, f := range strings.Fields(w.filters) {
			if !parse(route.allowed, "filter["+f+"]=x") {
				t.Errorf("%s: filter %q refused", route.path, f)
			}
		}
		for _, s := range strings.Fields(w.sorts) {
			if !parse(route.allowed, "sort="+s) || !parse(route.allowed, "sort=-"+s) {
				t.Errorf("%s: sort %q refused", route.path, s)
			}
		}
		if !parse(route.allowed, "") {
			t.Errorf("%s: paging alone refused", route.path)
		}
	}
	if !parse(nestedVolumeFields, "sort=-title") || parse(nestedFixedFields, "sort=title") {
		t.Error("nested sort rules wrong")
	}
}
