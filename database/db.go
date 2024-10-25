package database

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/sweetrpg/catalog-api/logging"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Db *mongo.Database
var client *mongo.Client

func SetupDatabase() {
	uri, found := os.LookupEnv("MONGODB_URI")
	if !found {
		logging.Logger.Fatal("'MONGODB_URI' environment variable not set!")
	}
	dbUrl, _ := url.Parse(uri)
	logging.Logger.Info("Connecting to database", "uri", dbUrl.Redacted())
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}

	dbName, found := os.LookupEnv("MONGODB_DB")
	if !found {
		logging.Logger.Fatal("'MONGODB_DB' environment variable not set!")
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

func Get(collection string, id string) (bson.D, error) {
	// TODO:

	return bson.D{}, nil
}

func Query(collection string, query bson.D, sortField string, start int, limit int) ([]any, error) {
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

	var results []any
	err = cursor.All(context.TODO(), &results)
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("Error while trying to fetch '%s' documents", collection), "error", err)
		return nil, err
	}

	return results, nil
}
