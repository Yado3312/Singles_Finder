package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type memoryRequestStore struct {
	records []mapRequestRecord
}

func (s *memoryRequestStore) ListRequests(context.Context) ([]mapRequestRecord, error) {
	return append([]mapRequestRecord(nil), s.records...), nil
}

func (s *memoryRequestStore) CreateRequest(_ context.Context, record mapRequestRecord) error {
	s.records = append(s.records, record)
	return nil
}

func (s *memoryRequestStore) JoinRequest(_ context.Context, id, accountID string) (mapRequestRecord, error) {
	for i := range s.records {
		record := &s.records[i]
		if record.ID != id {
			continue
		}
		if record.CreatorID == accountID {
			return mapRequestRecord{}, errOwnRequest
		}
		for _, participant := range record.Participants {
			if participant == accountID {
				return mapRequestRecord{}, errAlreadyJoined
			}
		}
		record.Participants = append(record.Participants, accountID)
		return *record, nil
	}
	return mapRequestRecord{}, errRequestNotFound
}

func (s *memoryRequestStore) Ping(context.Context) error { return nil }

func TestRemovedFictionalProfilesEndpoint(t *testing.T) {
	response := httptest.NewRecorder()
	routes(&memoryRequestStore{}, nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/profiles", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

type unavailableStore struct{ *memoryRequestStore }

func (unavailableStore) Ping(context.Context) error { return errors.New("database offline") }

func TestHealthReportsDatabaseFailure(t *testing.T) {
	response := httptest.NewRecorder()
	routes(unavailableStore{&memoryRequestStore{}}, nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}
