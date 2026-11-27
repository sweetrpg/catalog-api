package server

import (
	"net/http"
	"strconv"
	"testing"
)

// Another user's version must answer like a missing one on every route that retracts.
func TestRetractAnswersNotFoundForOtherUsersVersions(t *testing.T) {
	router := newVersionReadRouter(t)
	collections := map[string]string{
		"volumes": "volumes_versions", "publishers": "publishers_versions", "studios": "studios_versions",
		"persons": "persons_versions", "licenses": "licenses_versions",
	}
	for route, collection := range collections {
		id := seedVersionHistory(t, collection)
		actions := []string{"retract"}
		if route == "volumes" {
			actions = append(actions, "pull-back")
		}
		for _, action := range actions {
			post := func(version int, caller string) (int, string) {
				path := "/" + route + "/" + id + "/versions/" + strconv.Itoa(version) + "/" + action
				w := serveAs(router, http.MethodPost, path, "Bearer "+caller)
				return w.Code, w.Body.String()
			}
			for _, caller := range []string{"bob", "editor", "admin"} {
				t.Run(route+"/"+action+"/"+caller, func(t *testing.T) {
					missingCode, missingBody := post(99, caller)
					if missingCode != http.StatusNotFound {
						t.Fatalf("missing version: status %d, body %s", missingCode, missingBody)
					}
					for version := 1; version <= 6; version++ {
						if code, body := post(version, caller); code != http.StatusNotFound || body != missingBody {
							t.Errorf("version %d: status %d, body %s; want 404 %s", version, code, body, missingBody)
						}
					}
				})
			}
		}
		t.Run(route+"/owner", func(t *testing.T) {
			base := "/" + route + "/" + id + "/versions/"
			if w := serveAs(router, http.MethodPost, base+"1/retract", "Bearer alice"); w.Code != http.StatusBadRequest {
				t.Errorf("non-submitted: status %d, want 400, body %s", w.Code, w.Body)
			}
			if w := serveAs(router, http.MethodPost, base+"3/retract", "Bearer alice"); w.Code != http.StatusOK {
				t.Errorf("submitted: status %d, want 200, body %s", w.Code, w.Body)
			}
		})
	}
}
