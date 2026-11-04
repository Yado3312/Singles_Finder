package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoStore struct {
	collection *mongo.Collection
	database   *mongo.Database
	client     *mongo.Client
}

func connectMongo(ctx context.Context, uri, database string) (*mongo.Client, *mongoStore, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		return nil, nil, fmt.Errorf("connect to MongoDB: %w", err)
	}
	if err = client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("ping MongoDB: %w", err)
	}
	db := client.Database(database)
	collection := db.Collection("profiles")
	indexesByCollection := map[string][]mongo.IndexModel{
		"accounts": {
			{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"sessions": {
			{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			{Keys: bson.D{{Key: "accountId", Value: 1}}},
		},
		"profiles": {
			{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "accountId", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
			{Keys: bson.D{{Key: "interests", Value: 1}}},
		},
		"private_locations": {
			{Keys: bson.D{{Key: "accountId", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "point", Value: "2dsphere"}}},
			{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
		},
		"likes": {
			{Keys: bson.D{{Key: "fromAccountId", Value: 1}, {Key: "toAccountId", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "toAccountId", Value: 1}, {Key: "createdAt", Value: -1}}},
		},
		"matches": {
			{Keys: bson.D{{Key: "pairKey", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "accountIds", Value: 1}, {Key: "lastMessageAt", Value: -1}}},
		},
		"messages": {
			{Keys: bson.D{{Key: "matchId", Value: 1}, {Key: "createdAt", Value: -1}}},
		},
		"blocks": {
			{Keys: bson.D{{Key: "fromAccountId", Value: 1}, {Key: "toAccountId", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"reports": {
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
			{Keys: bson.D{{Key: "subjectAccountId", Value: 1}, {Key: "createdAt", Value: -1}}},
		},
	}
	for name, indexes := range indexesByCollection {
		if _, err = db.Collection(name).Indexes().CreateMany(ctx, indexes); err != nil {
			_ = client.Disconnect(context.Background())
			return nil, nil, fmt.Errorf("create %s indexes: %w", name, err)
		}
	}
	return client, &mongoStore{collection: collection, database: db, client: client}, nil
}

func (s mongoStore) List(ctx context.Context, interest, search string) ([]Profile, error) {
	filter := bson.D{}
	if interest != "" && interest != "Everyone" {
		filter = append(filter, bson.E{Key: "interests", Value: bson.Regex{Pattern: "^" + regexp.QuoteMeta(interest) + "$", Options: "i"}})
	}
	if search != "" {
		term := bson.Regex{Pattern: regexp.QuoteMeta(search), Options: "i"}
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{bson.D{{Key: "name", Value: term}}, bson.D{{Key: "role", Value: term}}, bson.D{{Key: "interests", Value: term}}}})
	}
	cursor, err := s.collection.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "id", Value: 1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	result := make([]Profile, 0)
	if err = cursor.All(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s mongoStore) ByID(ctx context.Context, id string) (Profile, error) {
	var profile Profile
	err := s.collection.FindOne(ctx, bson.D{{Key: "id", Value: id}}).Decode(&profile)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Profile{}, errNotFound
	}
	return profile, err
}

func (s mongoStore) SeedDemo(ctx context.Context) error {
	for _, p := range profiles {
		_, err := s.collection.UpdateOne(ctx, bson.D{{Key: "id", Value: p.ID}}, bson.D{{Key: "$setOnInsert", Value: p}}, options.UpdateOne().SetUpsert(true))
		if err != nil {
			return fmt.Errorf("seed demo profile %s: %w", p.ID, err)
		}
	}
	return nil
}

func (s mongoStore) Ping(ctx context.Context) error { return s.client.Ping(ctx, nil) }
