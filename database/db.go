package database

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-api/logging"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Db *mongo.Database
var client *mongo.Client

func SetupDatabase() {
	var dbUrl *url.URL
	var err error
	dbUri, found := os.LookupEnv(constants.DB_URI)
	dbName := os.Getenv(constants.DB_NAME)
	if found {
		dbUrl, err = url.Parse(dbUri)
	} else {
		dbScheme := os.Getenv(constants.DB_SCHEME)
		dbUser := os.Getenv(constants.DB_USER)
		dbPW := os.Getenv(constants.DB_PW)
		dbHost := os.Getenv(constants.DB_HOST)
		dbOpts := os.Getenv(constants.DB_OPTS)
		dbUrl = &url.URL{
			Scheme:     dbScheme,
			Host:       dbHost,
			User:       url.UserPassword(dbUser, dbPW),
			Path:       dbName,
			RawQuery:   dbOpts,
			ForceQuery: true,
		}
	}
	logging.Logger.Info("Connecting to database", "url", dbUrl.Redacted())
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(dbUrl.String()))
	if err != nil {
		panic(err)
	}

	logging.Logger.Info("Setting up database", "dbName", dbName)
	Db = client.Database(dbName)
}

func TeardownDatabase() {
	if client != nil {
		if err := client.Disconnect(context.TODO()); err != nil {
			logging.Logger.Error("Error while disconnecting from database", "error", err.Error())
		}
	}
}

func Get[T any](collection string, id string) (*T, error) {
	logging.Logger.Debug(fmt.Sprintf("Using '%s' collection on DB", collection))
	coll := Db.Collection(collection)
	logging.Logger.Debug(fmt.Sprintf("collection=%v", coll)) // TODO: remove

	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Unable to created ObjectID from %s: %s", id, err.Error()))
		return nil, err
	}
	filter := bson.D{{"_id", objectId}}
	var model T
	err = coll.FindOne(context.TODO(), filter).Decode(&model)
	// bsonBytes, err := bson.Marshal(result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}

		logging.Logger.Error(fmt.Sprintf("Failed to marshal BSON: %v", err))
		return nil, err
	}
	// logging.Logger.Debug(fmt.Sprintf("bsonBytes=%v", bsonBytes))

	// if err := bson.Unmarshal(bsonBytes, &model); err != nil {
	// 	logging.Logger.Error(fmt.Sprintf("Failed to unmarshal BSON to struct: %v", err))
	// 	return nil, err
	// }
	logging.Logger.Debug(fmt.Sprintf("model=%v", model))

	return &model, nil
}

func Query[T any](collection string, query bson.D, sortField string, start int, limit int) ([]*T, error) {
	logging.Logger.Debug(fmt.Sprintf("Using '%s' collection on DB", collection))
	coll := Db.Collection(collection)
	logging.Logger.Debug(fmt.Sprintf("collection=%v", coll)) // TODO: remove

	logging.Logger.Info(fmt.Sprintf("Querying for '%s'...", collection))
	sortStage := bson.D{{"$sort", bson.D{{sortField, 1}}}}
	logging.Logger.Debug(fmt.Sprintf("sort=%v", sortStage))
	skipStage := bson.D{{"$skip", start}}
	logging.Logger.Debug(fmt.Sprintf("skip=%v", skipStage))
	limitStage := bson.D{{"$limit", limit}}
	logging.Logger.Debug(fmt.Sprintf("limit=%v", limitStage))
	pipeline := mongo.Pipeline{sortStage, skipStage, limitStage}
	cursor, err := coll.Aggregate(context.TODO(), pipeline)
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while trying to find '%s' documents", collection), "error", err)
		return nil, err
	}

	var results []*T
	err = cursor.All(context.TODO(), &results)
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while trying to fetch '%s' documents", collection), "error", err)
		return nil, err
	}

	logging.Logger.Debug("query results", "result", results)
	var models []*T
	for _, r := range results {
		logging.Logger.Debug(fmt.Sprintf("r=%v", r))
		var model *T
		bsonBytes, err := bson.Marshal(r)
		if err != nil {
			logging.Logger.Error(fmt.Sprintf("Failed to marshal BSON: %v", err))
		}
		logging.Logger.Debug(fmt.Sprintf("bsonBytes=%v", bsonBytes))

		if err := bson.Unmarshal(bsonBytes, &model); err != nil {
			logging.Logger.Error(fmt.Sprintf("Failed to unmarshal BSON to struct: %v", err))
		}
		logging.Logger.Debug(fmt.Sprintf("model=%v", model))

		models = append(models, model)
	}
	// err = bson.Unmarshal(result, &licenses)

	return models, nil
}
