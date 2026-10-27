package main

import (
	"strings"
	"testing"
)

func TestReadConfigAcceptsAtlasURI(t *testing.T) {
	uri := "mongodb+srv://dbuser:p%40ssword@cluster0.gjyyl6k.mongodb.net/"
	t.Setenv("MONGODB_URI", uri)
	t.Setenv("DEMO_MODE", "false")
	cfg, err := readConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MongoURI != uri {
		t.Fatal("connection string changed")
	}
}

func TestReadConfigRejectsPlaceholders(t *testing.T) {
	t.Setenv("DEMO_MODE", "false")
	t.Setenv("MONGODB_URI", "mongodb+srv://<db_username>:<db_password>@cluster0.gjyyl6k.mongodb.net/")
	_, err := readConfig()
	if err == nil || !strings.Contains(err.Error(), "placeholders") {
		t.Fatalf("expected placeholder error, got %v", err)
	}
}

func TestReadConfigRequiresURI(t *testing.T) {
	t.Setenv("DEMO_MODE", "false")
	t.Setenv("MONGODB_URI", "")
	_, err := readConfig()
	if err == nil || !strings.Contains(err.Error(), "MONGODB_URI") {
		t.Fatalf("expected missing URI error, got %v", err)
	}
}
