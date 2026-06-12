package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"paap/internal/k8s"
)

func MongoDBURI(info k8s.MongoDBConnectionInfo) string {
	user := url.QueryEscape(info.Username)
	password := url.QueryEscape(info.Password)
	authDB := info.Database
	if authDB == "" {
		authDB = "admin"
	}
	return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s", user, password, info.Host, info.Port, url.PathEscape(authDB))
}

func ListMongoDBDatabases(ctx context.Context, info k8s.MongoDBConnectionInfo) ([]string, error) {
	client, err := openMongoDB(ctx, info)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)
	return client.ListDatabaseNames(ctx, bson.D{})
}

func ListMongoDBCollections(ctx context.Context, info k8s.MongoDBConnectionInfo, database string) ([]string, error) {
	client, err := openMongoDB(ctx, info)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)
	return client.Database(database).ListCollectionNames(ctx, bson.D{})
}

func CreateMongoDBCollection(ctx context.Context, info k8s.MongoDBConnectionInfo, database, collection string) error {
	client, err := openMongoDB(ctx, info)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)
	return client.Database(database).CreateCollection(ctx, collection)
}

func DropMongoDBCollection(ctx context.Context, info k8s.MongoDBConnectionInfo, database, collection string) error {
	client, err := openMongoDB(ctx, info)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)
	return client.Database(database).Collection(collection).Drop(ctx)
}

func PreviewMongoDBDocuments(ctx context.Context, info k8s.MongoDBConnectionInfo, database, collection string, limit int64) ([]string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	client, err := openMongoDB(ctx, info)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)
	cursor, err := client.Database(database).Collection(collection).Find(ctx, bson.D{}, options.Find().SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	result := make([]string, 0)
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		data, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}
		result = append(result, string(data))
	}
	return result, cursor.Err()
}

func InsertMongoDBDocument(ctx context.Context, info k8s.MongoDBConnectionInfo, database, collection, documentJSON string) error {
	doc, err := parseMongoJSON(documentJSON)
	if err != nil {
		return err
	}
	client, err := openMongoDB(ctx, info)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)
	_, err = client.Database(database).Collection(collection).InsertOne(ctx, doc)
	return err
}

func UpdateMongoDBDocuments(ctx context.Context, info k8s.MongoDBConnectionInfo, database, collection, filterJSON, updateJSON string) error {
	filter, err := parseMongoJSON(filterJSON)
	if err != nil {
		return err
	}
	update, err := parseMongoJSON(updateJSON)
	if err != nil {
		return err
	}
	client, err := openMongoDB(ctx, info)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)
	_, err = client.Database(database).Collection(collection).UpdateMany(ctx, filter, bson.M{"$set": update})
	return err
}

func DeleteMongoDBDocuments(ctx context.Context, info k8s.MongoDBConnectionInfo, database, collection, filterJSON string) error {
	filter, err := parseMongoJSON(filterJSON)
	if err != nil {
		return err
	}
	client, err := openMongoDB(ctx, info)
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)
	_, err = client.Database(database).Collection(collection).DeleteMany(ctx, filter)
	return err
}

func openMongoDB(ctx context.Context, info k8s.MongoDBConnectionInfo) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(MongoDBURI(info)).SetConnectTimeout(5 * time.Second))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return client, nil
}

func parseMongoJSON(raw string) (bson.M, error) {
	var result bson.M
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("json object is required")
	}
	return result, nil
}

func parseMongoLimit(value string) int64 {
	limit, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 20
	}
	return limit
}
