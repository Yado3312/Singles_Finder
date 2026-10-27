package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Run explicitly with CHECK_MONGODB=1 go test -run TestMongoConnection -v.
// This only pings MongoDB; it never creates indexes or writes data.
func TestMongoConnection(t *testing.T) {
	if os.Getenv("CHECK_MONGODB") != "1" {
		t.Skip("set CHECK_MONGODB=1 for the live connection check")
	}
	if err := loadEnvFile(); err != nil {
		t.Fatal(err)
	}
	cfg, err := readConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DemoMode {
		t.Fatal("DEMO_MODE must be false to check Atlas")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI).SetServerSelectionTimeout(10 * time.Second))
	if err != nil {
		t.Fatalf("MongoDB connection setup failed: %s", safeMongoError(err))
	}
	defer client.Disconnect(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("MongoDB ping failed: %s", safeMongoError(err))
	}
	names, err := client.Database(cfg.Database).ListCollectionNames(ctx, bson.D{})
	if err != nil {
		t.Fatalf("MongoDB database read failed: %s", safeMongoError(err))
	}
	count, err := client.Database(cfg.Database).Collection("profiles").CountDocuments(ctx, bson.D{})
	if err != nil {
		t.Fatalf("MongoDB profile read failed: %s", safeMongoError(err))
	}
	t.Logf("MongoDB ping OK; database %s readable; %d collections; %d profiles", cfg.Database, len(names), count)
}

func safeMongoError(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "authentication") || strings.Contains(message, "auth failed"):
		return "authentication failed; check the Atlas database user and password"
	case strings.Contains(message, "no such host") || strings.Contains(message, "dns"):
		return "DNS lookup failed; check the Atlas hostname and network"
	case strings.Contains(message, "timeout") || strings.Contains(message, "deadline") || strings.Contains(message, "server selection"):
		return "server not reachable; check Atlas IP access list and network"
	default:
		return "connection error; check URI formatting and Atlas access settings"
	}
}
