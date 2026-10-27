package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfilesFiltering(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/profiles?interest=Coffee&search=Sofia", nil)
	response := httptest.NewRecorder()
	routes(demoStore{}).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var result []Profile
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].Name != "Sofia" {
		t.Fatalf("unexpected profiles: %+v", result)
	}
}

func TestMissingProfile(t *testing.T) {
	response := httptest.NewRecorder()
	routes(demoStore{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/profiles/unknown", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

type unavailableStore struct{ demoStore }

func (unavailableStore) Ping(context.Context) error { return errors.New("database offline") }

func TestHealthReportsDatabaseFailure(t *testing.T) {
	response := httptest.NewRecorder()
	routes(unavailableStore{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}
