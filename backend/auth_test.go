package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type memoryAccountStore struct {
	accounts map[string]Account
	sessions map[string]authSession
}

func newMemoryAccountStore() *memoryAccountStore {
	return &memoryAccountStore{accounts: map[string]Account{}, sessions: map[string]authSession{}}
}

func (s *memoryAccountStore) CreateAccount(_ context.Context, account Account) error {
	if _, exists := s.accounts[account.Email]; exists {
		return errEmailTaken
	}
	s.accounts[account.Email] = account
	return nil
}

func (s *memoryAccountStore) FindByEmail(_ context.Context, email string) (Account, error) {
	account, ok := s.accounts[email]
	if !ok {
		return Account{}, errInvalidCredentials
	}
	return account, nil
}

func (s *memoryAccountStore) GetAccount(_ context.Context, id string) (Account, error) {
	for _, account := range s.accounts {
		if account.ID == id {
			return account, nil
		}
	}
	return Account{}, errInvalidSession
}

func (s *memoryAccountStore) UpdateQuiz(_ context.Context, id string, input quizRequest) error {
	for email, account := range s.accounts {
		if account.ID == id {
			account.Interests = input.Interests
			account.Personality = input.Personality
			account.DatePace = input.DatePace
			account.QuizComplete = true
			s.accounts[email] = account
			return nil
		}
	}
	return errInvalidSession
}

func (s *memoryAccountStore) UpdateName(_ context.Context, id, name string) error {
	for email, account := range s.accounts {
		if account.ID == id {
			account.Name = name
			s.accounts[email] = account
			return nil
		}
	}
	return errInvalidSession
}

func (s *memoryAccountStore) CreateSession(_ context.Context, session authSession) error {
	s.sessions[session.TokenHash] = session
	return nil
}

func (s *memoryAccountStore) GetSession(_ context.Context, tokenHash string) (authSession, error) {
	session, ok := s.sessions[tokenHash]
	if !ok {
		return authSession{}, errInvalidSession
	}
	return session, nil
}

func (s *memoryAccountStore) DeleteSession(_ context.Context, tokenHash string) error {
	delete(s.sessions, tokenHash)
	return nil
}

func TestRegisterQuizAndProtectedRequests(t *testing.T) {
	store := newMemoryAccountStore()
	handler := routes(&memoryRequestStore{}, store)
	register := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(
		`{"name":"Ana","email":" ANA@example.com ","password":"secure-password-123","birthDate":"2000-01-01"}`,
	))
	register.Header.Set("Content-Type", "application/json")
	registered := httptest.NewRecorder()
	handler.ServeHTTP(registered, register)
	if registered.Code != http.StatusOK {
		t.Fatalf("register status = %d, body = %s", registered.Code, registered.Body.String())
	}
	cookie := registered.Result().Cookies()[0]
	if cookie.Name != sessionCookie || !cookie.HttpOnly || cookie.Value == "" {
		t.Fatalf("unexpected session cookie: %+v", cookie)
	}
	if strings.Contains(registered.Body.String(), "passwordHash") {
		t.Fatal("account response exposed password hash")
	}

	duplicate := httptest.NewRecorder()
	handler.ServeHTTP(duplicate, httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(
		`{"name":"Ana","email":"ana@example.com","password":"secure-password-123","birthDate":"2000-01-01"}`,
	)))
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d", duplicate.Code)
	}

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/requests", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated requests status = %d", unauthorized.Code)
	}

	quiz := httptest.NewRequest(http.MethodPut, "/api/auth/quiz", strings.NewReader(
		`{"interests":["Nature","Music","Coffee"],"personality":"Adventurous","datePace":"Balanced"}`,
	))
	quiz.AddCookie(cookie)
	savedQuiz := httptest.NewRecorder()
	handler.ServeHTTP(savedQuiz, quiz)
	if savedQuiz.Code != http.StatusOK {
		t.Fatalf("quiz status = %d, body = %s", savedQuiz.Code, savedQuiz.Body.String())
	}
	var response accountResponse
	if err := json.Unmarshal(savedQuiz.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.QuizComplete || len(response.Interests) != 3 || response.Email != "ana@example.com" {
		t.Fatalf("unexpected saved account response: %+v", response)
	}

	authorized := httptest.NewRecorder()
	mapRequest := httptest.NewRequest(http.MethodGet, "/api/requests", nil)
	mapRequest.AddCookie(cookie)
	handler.ServeHTTP(authorized, mapRequest)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authenticated requests status = %d", authorized.Code)
	}

	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(
		`{"email":"ana@example.com","password":"secure-password-123"}`,
	))
	loggedIn := httptest.NewRecorder()
	handler.ServeHTTP(loggedIn, login)
	if loggedIn.Code != http.StatusOK || len(loggedIn.Result().Cookies()) == 0 {
		t.Fatalf("login status = %d, body = %s", loggedIn.Code, loggedIn.Body.String())
	}
	logout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logout.AddCookie(loggedIn.Result().Cookies()[0])
	loggedOut := httptest.NewRecorder()
	handler.ServeHTTP(loggedOut, logout)
	if loggedOut.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", loggedOut.Code)
	}
	currentAccount := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	currentAccount.AddCookie(loggedIn.Result().Cookies()[0])
	afterLogout := httptest.NewRecorder()
	handler.ServeHTTP(afterLogout, currentAccount)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("session remained valid after logout: %d", afterLogout.Code)
	}
}

func TestRegistrationRequiresAdultAndValidQuiz(t *testing.T) {
	if _, valid := validBirthDate(time.Now().AddDate(-17, 0, 0).Format("2006-01-02"), time.Now()); valid {
		t.Fatal("underage birth date was accepted")
	}
	if _, valid := validBirthDate(time.Now().AddDate(-18, 0, 1).Format("2006-01-02"), time.Now()); valid {
		t.Fatal("date that has not reached 18 was accepted")
	}
	validInput := quizRequest{
		Interests:   []string{"Art", "Coffee", "Travel"},
		Personality: "Social",
		DatePace:    "Slow & steady",
	}
	if !validQuiz(validInput) {
		t.Fatal("valid test answers were rejected")
	}
	validInput.Interests = []string{"Art", "Art", "Coffee"}
	if validQuiz(validInput) {
		t.Fatal("duplicate interests were accepted")
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	store := newMemoryAccountStore()
	account := Account{ID: "account-1", Email: "person@example.com", Status: "active"}
	store.accounts[account.Email] = account
	token := "expired-token"
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])
	store.sessions[tokenHash] = authSession{TokenHash: tokenHash, AccountID: account.ID, ExpiresAt: time.Now().Add(-time.Minute)}
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	_, _, err := accountForRequest(context.Background(), request, store)
	if err == nil {
		t.Fatal("expired session was accepted")
	}
}
