// Command backfill-canonical-user-ids rewrites legacy Auth0-subject values in the *_meta and
// *_versions audit fields (created_by, updated_by, submitted_by, reviewed_by) to the canonical
// users._id each subject maps to, using users-api's internal resolve-subjects batch endpoint.
// Subjects that cannot be resolved to a user become the "system" actor. Idempotent: values that
// are already 24-hex canonical ids or "system" are skipped. Dry-run by default; pass -apply to
// write. Run once after the catalog-api release that adopts authz.Viewer.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/joho/godotenv"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/common.go/logging"
	"github.com/sweetrpg/common.go/util"
	"github.com/sweetrpg/mongodb.go/database"
	"go.mongodb.org/mongo-driver/bson"
)

type target struct {
	collection string
	field      string
}

var targets = []target{
	{"volumes_meta", "created_by"}, {"volumes_meta", "updated_by"},
	{"publishers_meta", "created_by"}, {"publishers_meta", "updated_by"},
	{"studios_meta", "created_by"}, {"studios_meta", "updated_by"},
	{"persons_meta", "created_by"}, {"persons_meta", "updated_by"},
	{"licenses_meta", "created_by"}, {"licenses_meta", "updated_by"},
	{"volumes_versions", "submitted_by"}, {"volumes_versions", "reviewed_by"},
	{"publishers_versions", "submitted_by"}, {"publishers_versions", "reviewed_by"},
	{"studios_versions", "submitted_by"}, {"studios_versions", "reviewed_by"},
	{"persons_versions", "submitted_by"}, {"persons_versions", "reviewed_by"},
	{"licenses_versions", "submitted_by"}, {"licenses_versions", "reviewed_by"},
}

var canonicalIDRE = regexp.MustCompile(`^[0-9a-f]{24}$`)

type resolveSubjectsRequest struct {
	Subjects []string `json:"subjects"`
}

func main() {
	_ = godotenv.Load(".env")
	logging.Init()

	apply := flag.Bool("apply", false, "write changes; default is a dry run")
	adminToken := flag.String("users-admin-token", os.Getenv("USERS_ADMIN_TOKEN"), "admin bearer token for users-api's internal resolve-subjects endpoint")
	flag.Parse()

	database.SetupDatabase()
	defer database.TeardownDatabase()

	usersBaseURL := util.GetEnv(constants.USERS_API_URL, "")

	ctx := context.Background()

	distinct := map[string]struct{}{}
	for _, t := range targets {
		coll := database.Db.Collection(t.collection)
		values, err := coll.Distinct(ctx, t.field, bson.M{})
		if err != nil {
			logging.Logger.Error("backfill: distinct failed", "collection", t.collection, "field", t.field, "error", err.Error())
			return
		}
		for _, v := range values {
			s, ok := v.(string)
			if !ok || s == "" || s == "system" || canonicalIDRE.MatchString(s) {
				continue
			}
			distinct[s] = struct{}{}
		}
	}
	if len(distinct) == 0 {
		logging.Logger.Info("backfill: no subject-shaped values found to resolve")
		return
	}
	logging.Logger.Info("backfill: distinct subjects found", "count", len(distinct))

	subjects := make([]string, 0, len(distinct))
	for s := range distinct {
		subjects = append(subjects, s)
	}

	resolved := map[string]string{}
	if *adminToken == "" {
		logging.Logger.Warn("backfill: no users-admin-token set, skipping subject resolution", "dry_run", !*apply)
	} else {
		var err error
		resolved, err = resolveSubjects(ctx, usersBaseURL, *adminToken, subjects)
		if err != nil {
			logging.Logger.Error("backfill: resolve subjects failed", "error", err.Error())
			return
		}
	}

	rewritten := map[string]string{}
	for _, s := range subjects {
		if id := resolved[s]; id != "" {
			rewritten[s] = id
		} else {
			logging.Logger.Warn("backfill: unmappable subject -> system", "subject", s)
			rewritten[s] = "system"
		}
	}

	var updated int64
	for _, t := range targets {
		coll := database.Db.Collection(t.collection)
		for oldValue, newValue := range rewritten {
			modifier := "would update"
			affected := int64(0)
			if *apply {
				res, err := coll.UpdateMany(ctx, bson.M{t.field: oldValue}, bson.D{{Key: "$set", Value: bson.D{{Key: t.field, Value: newValue}}}})
				if err != nil {
					logging.Logger.Error("backfill: update failed", "collection", t.collection, "field", t.field, "value", oldValue, "error", err.Error())
					return
				}
				modifier = "updated"
				affected = res.ModifiedCount
				updated += affected
			} else {
				count, err := coll.CountDocuments(ctx, bson.M{t.field: oldValue})
				if err != nil {
					logging.Logger.Error("backfill: count failed", "collection", t.collection, "field", t.field, "value", oldValue, "error", err.Error())
					return
				}
				affected = count
			}
			if affected > 0 {
				logging.Logger.Info("backfill: "+modifier, "collection", t.collection, "field", t.field, "value", oldValue, "replacement", newValue, "documents", affected)
			}
		}
	}

	if *apply {
		logging.Logger.Info("backfill: complete", "documents_updated", updated)
	} else {
		fmt.Println("dry run complete - pass -apply to write changes; still remaining check via task 4.4")
	}
}

func resolveSubjects(ctx context.Context, usersBaseURL, token string, subjects []string) (map[string]string, error) {
	if usersBaseURL == "" {
		return nil, fmt.Errorf("USERS_API_URL is not set")
	}

	body, err := json.Marshal(resolveSubjectsRequest{Subjects: subjects})
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, usersBaseURL+"/internal/resolve-subjects", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resolve-subjects returned %s", resp.Status)
	}

	out := map[string]string{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
