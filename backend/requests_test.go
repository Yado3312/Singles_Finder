package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testSession(t *testing.T, store *memoryAccountStore, account Account) *http.Cookie {
	t.Helper()
	store.accounts[account.Email] = account
	token := account.ID + "-test-session"
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	store.sessions[hash] = authSession{TokenHash: hash, AccountID: account.ID, ExpiresAt: time.Now().Add(time.Hour)}
	return &http.Cookie{Name: sessionCookie, Value: token}
}

func TestMapRequestCreateAndJoin(t *testing.T) {
	accounts := newMemoryAccountStore()
	creatorCookie := testSession(t, accounts, Account{
		ID: "creator", Name: "Ana", Email: "ana@example.com", Interests: []string{"Art", "Coffee"},
		Status: "active", QuizComplete: true,
	})
	joinerCookie := testSession(t, accounts, Account{
		ID: "joiner", Name: "Luis", Email: "luis@example.com", Status: "active", QuizComplete: true,
	})
	store := &memoryRequestStore{}
	handler := routes(store, accounts)

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/requests", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status = %d", unauthorized.Code)
	}

	create := httptest.NewRequest(http.MethodPost, "/api/requests",
		strings.NewReader(`{"description":"Tomar café y conversar este sábado","lat":28.63456,"lng":-106.08123}`))
	create.AddCookie(creatorCookie)
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var response publicMapRequest
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.CreatorName != "Ana" || !response.IsCreator || !response.JoinedByMe ||
		response.ParticipantCount != 1 || response.Lat != 28.635 || response.Lng != -106.081 {
		t.Fatalf("unexpected created request response: %+v", response)
	}
	if strings.Contains(created.Body.String(), "creatorId") || strings.Contains(created.Body.String(), "participants") {
		t.Fatalf("private request fields leaked: %s", created.Body.String())
	}

	join := httptest.NewRequest(http.MethodPost, "/api/requests/"+response.ID+"/join", nil)
	join.AddCookie(joinerCookie)
	joined := httptest.NewRecorder()
	handler.ServeHTTP(joined, join)
	if joined.Code != http.StatusOK {
		t.Fatalf("join status = %d, body = %s", joined.Code, joined.Body.String())
	}
	if err := json.Unmarshal(joined.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.JoinedByMe || response.ParticipantCount != 2 || response.CreatorName != "Ana" {
		t.Fatalf("unexpected joined request response: %+v", response)
	}

	duplicate := httptest.NewRecorder()
	handler.ServeHTTP(duplicate, join)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate join status = %d", duplicate.Code)
	}
	ownJoin := httptest.NewRequest(http.MethodPost, "/api/requests/"+response.ID+"/join", nil)
	ownJoin.AddCookie(creatorCookie)
	ownJoined := httptest.NewRecorder()
	handler.ServeHTTP(ownJoined, ownJoin)
	if ownJoined.Code != http.StatusConflict {
		t.Fatalf("own request join status = %d", ownJoined.Code)
	}

	list := httptest.NewRequest(http.MethodGet, "/api/requests", nil)
	list.AddCookie(joinerCookie)
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, list)
	var requests []publicMapRequest
	if err := json.Unmarshal(listed.Body.Bytes(), &requests); err != nil {
		t.Fatal(err)
	}
	if listed.Code != http.StatusOK || len(requests) != 1 || !requests[0].JoinedByMe {
		t.Fatalf("unexpected requests list: status %d, %+v", listed.Code, requests)
	}
}

func TestMapRequestRejectsInvalidDescriptionAndLocation(t *testing.T) {
	accounts := newMemoryAccountStore()
	cookie := testSession(t, accounts, Account{
		ID: "creator", Name: "Ana", Email: "ana@example.com", Status: "active", QuizComplete: true,
	})
	handler := routes(&memoryRequestStore{}, accounts)
	for _, body := range []string{
		`{"description":"too short","lat":28.6,"lng":-106.0}`,
		`{"description":"A valid plan description","lat":91,"lng":-106.0}`,
		`{"description":"A valid plan description","lat":28.6,"lng":181}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/requests", strings.NewReader(body))
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, response.Code)
		}
	}
}
