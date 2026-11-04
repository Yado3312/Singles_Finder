package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 30 * 24 * time.Hour
const sessionCookie = "singlesfinder_session"

var (
	errEmailTaken         = errors.New("email already registered")
	errInvalidCredentials = errors.New("invalid email or password")
	errInvalidSession     = errors.New("invalid session")
)

type accountStore interface {
	CreateAccount(context.Context, Account) error
	FindByEmail(context.Context, string) (Account, error)
	GetAccount(context.Context, string) (Account, error)
	UpdateQuiz(context.Context, string, quizRequest) error
	UpdateName(context.Context, string, string) error
	CreateSession(context.Context, authSession) error
	GetSession(context.Context, string) (authSession, error)
	DeleteSession(context.Context, string) error
}

type accountResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Interests    []string  `json:"interests"`
	Personality  string    `json:"personality"`
	DatePace     string    `json:"datePace"`
	QuizComplete bool      `json:"quizComplete"`
	CreatedAt    time.Time `json:"createdAt"`
}

type registerRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	BirthDate string `json:"birthDate"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type quizRequest struct {
	Interests   []string `json:"interests"`
	Personality string   `json:"personality"`
	DatePace    string   `json:"datePace"`
}

type updateNameRequest struct {
	Name string `json:"name"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func normalizeEmail(email string) (string, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 {
		return "", false
	}
	parsed, err := mail.ParseAddress(email)
	return email, err == nil && parsed.Address == email
}

func validBirthDate(value string, now time.Time) (time.Time, bool) {
	birthDate, err := time.Parse("2006-01-02", value)
	if err != nil || birthDate.After(now) {
		return time.Time{}, false
	}
	age := now.Year() - birthDate.Year()
	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}
	return birthDate, age >= 18 && age <= 120
}

func newSessionToken() (string, string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", "", err
	}
	raw := hex.EncodeToString(token)
	hash := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(hash[:]), nil
}

func accountView(account Account) accountResponse {
	return accountResponse{
		ID: account.ID, Name: account.Name, Email: account.Email,
		Interests: account.Interests, Personality: account.Personality,
		DatePace: account.DatePace, QuizComplete: account.QuizComplete,
		CreatedAt: account.CreatedAt,
	}
}

func writeAccountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errEmailTaken):
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "email already registered"})
	case errors.Is(err, errInvalidCredentials):
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "email or password is incorrect"})
	case errors.Is(err, errInvalidSession):
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	default:
		log.Printf("account operation failed: %v", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "account service unavailable"})
	}
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: expires,
		MaxAge: int(time.Until(expires).Seconds()), HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
}

func accountForRequest(ctx context.Context, r *http.Request, store accountStore) (Account, string, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return Account{}, "", errInvalidSession
	}
	sum := sha256.Sum256([]byte(cookie.Value))
	tokenHash := hex.EncodeToString(sum[:])
	session, err := store.GetSession(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, errInvalidSession) {
			return Account{}, tokenHash, errInvalidSession
		}
		return Account{}, tokenHash, err
	}
	if time.Now().After(session.ExpiresAt) {
		return Account{}, tokenHash, errInvalidSession
	}
	account, err := store.GetAccount(ctx, session.AccountID)
	if err != nil {
		if errors.Is(err, errInvalidSession) {
			return Account{}, tokenHash, errInvalidSession
		}
		return Account{}, tokenHash, err
	}
	if account.Status != "active" {
		return Account{}, tokenHash, errInvalidSession
	}
	return account, tokenHash, nil
}

func registerAuthRoutes(mux *http.ServeMux, store accountStore) {
	mux.HandleFunc("POST /api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeAccountError(w, errors.New("account service unavailable"))
			return
		}
		var input registerRequest
		if err := decodeJSON(w, r, &input); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		name := strings.TrimSpace(input.Name)
		email, validEmail := normalizeEmail(input.Email)
		birthDate, adult := validBirthDate(input.BirthDate, time.Now())
		if len(name) < 2 || len(name) > 80 || !validEmail || len(input.Password) < 10 || len(input.Password) > 72 || !adult {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "enter a name, valid email, password of at least 10 characters, and birth date confirming you are 18 or older"})
			return
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			writeAccountError(w, err)
			return
		}
		idBytes := make([]byte, 16)
		if _, err := rand.Read(idBytes); err != nil {
			writeAccountError(w, err)
			return
		}
		now := time.Now().UTC()
		account := Account{
			ID: hex.EncodeToString(idBytes), Name: name, Email: email,
			PasswordHash: string(passwordHash), BirthDate: birthDate.UTC(),
			Status: "active", CreatedAt: now, UpdatedAt: now,
		}
		if err := store.CreateAccount(r.Context(), account); err != nil {
			writeAccountError(w, err)
			return
		}
		issueSession(w, r, store, account)
	})

	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeAccountError(w, errors.New("account service unavailable"))
			return
		}
		var input loginRequest
		if err := decodeJSON(w, r, &input); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		email, validEmail := normalizeEmail(input.Email)
		if !validEmail || len(input.Password) == 0 || len(input.Password) > 72 {
			writeAccountError(w, errInvalidCredentials)
			return
		}
		account, err := store.FindByEmail(r.Context(), email)
		if errors.Is(err, errInvalidCredentials) {
			writeAccountError(w, errInvalidCredentials)
			return
		}
		if err != nil {
			writeAccountError(w, err)
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(input.Password)) != nil || account.Status != "active" {
			writeAccountError(w, errInvalidCredentials)
			return
		}
		issueSession(w, r, store, account)
	})

	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeAccountError(w, errors.New("account service unavailable"))
			return
		}
		account, _, err := accountForRequest(r.Context(), r, store)
		if err != nil {
			writeAccountError(w, err)
			return
		}
		jsonResponse(w, http.StatusOK, accountView(account))
	})

	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if store != nil {
			if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
				sum := sha256.Sum256([]byte(cookie.Value))
				hash := hex.EncodeToString(sum[:])
				if err := store.DeleteSession(r.Context(), hash); err != nil {
					writeAccountError(w, err)
					return
				}
			}
		}
		clearSessionCookie(w, r)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("PUT /api/auth/quiz", func(w http.ResponseWriter, r *http.Request) {
		account, err := requireAccount(w, r, store)
		if err != nil {
			return
		}
		var input quizRequest
		if err := decodeJSON(w, r, &input); err != nil || !validQuiz(input) {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "complete the personality test with valid choices"})
			return
		}
		if err := store.UpdateQuiz(r.Context(), account.ID, input); err != nil {
			writeAccountError(w, err)
			return
		}
		account.Interests = input.Interests
		account.Personality = input.Personality
		account.DatePace = input.DatePace
		account.QuizComplete = true
		jsonResponse(w, http.StatusOK, accountView(account))
	})

	mux.HandleFunc("PUT /api/auth/settings", func(w http.ResponseWriter, r *http.Request) {
		account, err := requireAccount(w, r, store)
		if err != nil {
			return
		}
		var input updateNameRequest
		if err := decodeJSON(w, r, &input); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		name := strings.TrimSpace(input.Name)
		if len(name) < 2 || len(name) > 80 {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "name must be between 2 and 80 characters"})
			return
		}
		if err := store.UpdateName(r.Context(), account.ID, name); err != nil {
			writeAccountError(w, err)
			return
		}
		account.Name = name
		jsonResponse(w, http.StatusOK, accountView(account))
	})
}

func issueSession(w http.ResponseWriter, r *http.Request, store accountStore, account Account) {
	token, tokenHash, err := newSessionToken()
	if err != nil {
		writeAccountError(w, err)
		return
	}
	expires := time.Now().Add(sessionDuration)
	if err := store.CreateSession(r.Context(), authSession{TokenHash: tokenHash, AccountID: account.ID, ExpiresAt: expires}); err != nil {
		writeAccountError(w, err)
		return
	}
	setSessionCookie(w, r, token, expires)
	jsonResponse(w, http.StatusOK, accountView(account))
}

func requireAccount(w http.ResponseWriter, r *http.Request, store accountStore) (Account, error) {
	if store == nil {
		writeAccountError(w, errors.New("account service unavailable"))
		return Account{}, errInvalidSession
	}
	account, _, err := accountForRequest(r.Context(), r, store)
	if err != nil {
		writeAccountError(w, err)
		return Account{}, err
	}
	return account, nil
}

func allowDiscoveryRequest(w http.ResponseWriter, r *http.Request, store accountStore) bool {
	if store == nil {
		return true
	}
	if _, _, err := accountForRequest(r.Context(), r, store); err != nil {
		writeAccountError(w, err)
		return false
	}
	return true
}

func validQuiz(input quizRequest) bool {
	allowedInterests := map[string]bool{
		"Nature": true, "Music": true, "Art": true, "Coffee": true,
		"Travel": true, "Cooking": true, "Reading": true, "Sports": true,
		"Video games": true, "Animals": true, "Movies": true, "Volunteering": true,
	}
	if len(input.Interests) < 3 || len(input.Interests) > 8 {
		return false
	}
	seen := make(map[string]bool, len(input.Interests))
	for _, interest := range input.Interests {
		if !allowedInterests[interest] || seen[interest] {
			return false
		}
		seen[interest] = true
	}
	validPersonality := input.Personality == "Easygoing" || input.Personality == "Adventurous" || input.Personality == "Social"
	validPace := input.DatePace == "Slow & steady" || input.DatePace == "Balanced" || input.DatePace == "Spontaneous"
	return validPersonality && validPace
}
