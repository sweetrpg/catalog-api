package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/catalog-api/assets"
	"github.com/sweetrpg/catalog-api/cachettl"
	"github.com/sweetrpg/catalog-api/editsession"
	catalogmodels "github.com/sweetrpg/catalog-objects.go/models"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// newVersionReadRouter wires all five types against a fake auth-api where the bearer token
// names the caller.
func newVersionReadRouter(t *testing.T) *gin.Engine {
	t.Helper()
	roles := map[string]string{
		"alice": authz.RoleSubmitter, "bob": authz.RoleSubmitter, "plain": authz.RoleUser,
		"editor": authz.RoleEditor, "admin": authz.RoleAdmin, "denied": authz.RoleEditor,
	}
	authAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		role, known := roles[token]
		switch {
		case token == "outage":
			w.WriteHeader(http.StatusBadGateway)
		case !known:
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/profile":
			_ = json.NewEncoder(w).Encode(map[string]string{"user_id": token})
		default:
			_ = json.NewEncoder(w).Encode(authz.CheckResponse{Allowed: token != "denied", Roles: []string{role}, Sub: token})
		}
	}))
	t.Cleanup(authAPI.Close)

	client := authz.NewClient(authAPI.URL, authAPI.URL)
	store, ttls := persistence.NewInMemoryStore(0), cachettl.Config{}
	r := gin.New()
	setupVolumeHandlers(r, store, ttls, client, assets.NewClient(""), editsession.NewStore(newTestRedisPool(t)), nil)
	setupPublisherHandlers(r, store, ttls, client, nil)
	setupStudioHandlers(r, store, ttls, client, nil)
	setupPersonHandlers(r, store, ttls, client, nil)
	setupLicenseHandlers(r, store, ttls, client, nil)
	return r
}

// seedVersionHistory stores one version per state, all submitted by alice: version 1 is live,
// 2 archived, and 3-6 never published.
func seedVersionHistory(t *testing.T, collection string) string {
	t.Helper()
	recordID := "reader-" + primitive.NewObjectID().Hex()
	for i, state := range []catalogmodels.VersionState{
		catalogmodels.VersionStateLive, catalogmodels.VersionStateArchived,
		catalogmodels.VersionStateSubmitted, catalogmodels.VersionStateRejected,
		catalogmodels.VersionStatePartiallyAccepted, catalogmodels.VersionStateWithdrawn,
	} {
		_, err := database.Db.Collection(collection).InsertOne(t.Context(), bson.M{
			"_id": primitive.NewObjectID().Hex(), "record_id": recordID, "version": i + 1,
			"state": string(state), "submitted_by": "alice", "reviewed_by": "rita-reviewer",
			"review_note": "private-note", "staged_cover_asset_id": "staged-alice",
		})
		if err != nil {
			t.Fatalf("seed version: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = database.Db.Collection(collection).DeleteMany(context.Background(), bson.M{"record_id": recordID})
	})
	return recordID
}

func serveAs(r *gin.Engine, method, path, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestVersionRoutesAreScopedToTheCaller(t *testing.T) {
	router := newVersionReadRouter(t)
	collections := map[string]string{
		"volumes": "volumes_versions", "publishers": "publishers_versions", "studios": "studios_versions",
		"persons": "persons_versions", "licenses": "licenses_versions",
	}
	const public, all = 2, 6
	cases := []struct {
		name, authorization string
		visible             int
		own, reviewer       bool
	}{
		{name: "signed out", visible: public},
		{name: "invalid token", authorization: "Bearer forged", visible: public},
		{name: "not a bearer token", authorization: "Basic YWxpY2U6eA==", visible: public},
		{name: "auth unavailable", authorization: "Bearer outage", visible: public},
		{name: "denied caller with a review role", authorization: "Bearer denied", visible: public},
		{name: "another submitter", authorization: "Bearer bob", visible: public},
		{name: "user without a write role", authorization: "Bearer plain", visible: public},
		{name: "submitter", authorization: "Bearer alice", visible: all, own: true},
		{name: "editor", authorization: "Bearer editor", visible: all, reviewer: true},
		{name: "admin", authorization: "Bearer admin", visible: all, reviewer: true},
	}

	for route, collection := range collections {
		id := seedVersionHistory(t, collection)
		for _, tc := range cases {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				base := "/" + route + "/" + id + "/versions"
				list := serveAs(router, http.MethodGet, base, tc.authorization)
				var listed []map[string]any
				if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || list.Code != http.StatusOK {
					t.Fatalf("list: status %d, error %v, body %s", list.Code, err, list.Body)
				}
				if len(listed) != tc.visible {
					t.Errorf("listed %d versions, want %d", len(listed), tc.visible)
				}
				assertNotShared(t, list)
				assertIdentityFields(t, list.Body.String(), route, tc.own, tc.reviewer)

				for version := 1; version <= all; version++ {
					got := serveAs(router, http.MethodGet, base+"/"+strconv.Itoa(version), tc.authorization)
					want := http.StatusOK
					if version > tc.visible {
						want = http.StatusNotFound
					}
					if got.Code != want {
						t.Errorf("version %d: status %d, want %d", version, got.Code, want)
						continue
					}
					assertNotShared(t, got)
					if want == http.StatusOK {
						assertIdentityFields(t, got.Body.String(), route, tc.own, tc.reviewer)
					}
				}
			})
		}
	}
}

func assertNotShared(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if cc, vary := w.Header().Get("Cache-Control"), w.Header().Get("Vary"); cc != "private, no-store" || vary != "Authorization" {
		t.Errorf("Cache-Control %q, Vary %q", cc, vary)
	}
}

func assertIdentityFields(t *testing.T, body, route string, own, reviewer bool) {
	t.Helper()
	want := map[string]bool{"alice": own || reviewer, "private-note": own || reviewer, "rita-reviewer": reviewer}
	if route == "volumes" {
		want["staged-alice"] = own || reviewer
	}
	for value, present := range want {
		// Quoted so the record ID or a field name can't match.
		if strings.Contains(body, `"`+value+`"`) != present {
			t.Errorf("%q present = %v, want %v", value, !present, present)
		}
	}
}

func TestPullBackOfAnotherUsersVersionIsNotFound(t *testing.T) {
	router := newVersionReadRouter(t)
	id := seedVersionHistory(t, "volumes_versions")

	w := serveAs(router, http.MethodPost, "/volumes/"+id+"/versions/3/pull-back", "Bearer bob")

	if w.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", w.Code)
	}
}
