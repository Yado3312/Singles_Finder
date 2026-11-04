package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoStore struct {
	database *mongo.Database
	client   *mongo.Client
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
	indexesByCollection := map[string][]mongo.IndexModel{
		"accounts": {
			{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"sessions": {
			{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			{Keys: bson.D{{Key: "accountId", Value: 1}}},
		},
		"requests": {
			{Keys: bson.D{{Key: "createdAt", Value: -1}}},
			{Keys: bson.D{{Key: "creatorId", Value: 1}, {Key: "createdAt", Value: -1}}},
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
	return client, &mongoStore{database: db, client: client}, nil
}

func (s mongoStore) Ping(ctx context.Context) error { return s.client.Ping(ctx, nil) }

func (s mongoStore) ListRequests(ctx context.Context) ([]mapRequestRecord, error) {
	cursor, err := s.database.Collection("requests").Find(ctx, bson.D{}, options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(200))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var records []mapRequestRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s mongoStore) CreateRequest(ctx context.Context, record mapRequestRecord) error {
	_, err := s.database.Collection("requests").InsertOne(ctx, record)
	return err
}

func (s mongoStore) JoinRequest(ctx context.Context, id, accountID string) (mapRequestRecord, error) {
	collection := s.database.Collection("requests")
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "creatorId", Value: bson.D{{Key: "$ne", Value: accountID}}},
		{Key: "participants", Value: bson.D{{Key: "$ne", Value: accountID}}},
	}
	var record mapRequestRecord
	err := collection.FindOneAndUpdate(ctx, filter,
		bson.D{{Key: "$addToSet", Value: bson.D{{Key: "participants", Value: accountID}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&record)
	if err == nil {
		return record, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return mapRequestRecord{}, err
	}
	err = collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&record)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return mapRequestRecord{}, errRequestNotFound
	}
	if err != nil {
		return mapRequestRecord{}, err
	}
	if record.CreatorID == accountID {
		return mapRequestRecord{}, errOwnRequest
	}
	return mapRequestRecord{}, errAlreadyJoined
}
