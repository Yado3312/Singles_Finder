package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

var (
	errRequestNotFound = errors.New("request not found")
	errOwnRequest      = errors.New("cannot join own request")
	errAlreadyJoined   = errors.New("already joined")
)

type mapRequestRecord struct {
	ID               string    `bson:"_id" json:"id"`
	CreatorID        string    `bson:"creatorId" json:"-"`
	CreatorName      string    `bson:"creatorName" json:"creatorName"`
	CreatorInterests []string  `bson:"creatorInterests" json:"creatorInterests"`
	Description      string    `bson:"description" json:"description"`
	Lat              float64   `bson:"lat" json:"lat"`
	Lng              float64   `bson:"lng" json:"lng"`
	Participants     []string  `bson:"participants" json:"-"`
	CreatedAt        time.Time `bson:"createdAt" json:"createdAt"`
}

type publicMapRequest struct {
	ID               string    `json:"id"`
	CreatorName      string    `json:"creatorName"`
	CreatorInterests []string  `json:"creatorInterests"`
	Description      string    `json:"description"`
	Lat              float64   `json:"lat"`
	Lng              float64   `json:"lng"`
	CreatedAt        time.Time `json:"createdAt"`
	ParticipantCount int       `json:"participantCount"`
	JoinedByMe       bool      `json:"joinedByMe"`
	IsCreator        bool      `json:"isCreator"`
}

type createMapRequestInput struct {
	Description string  `json:"description"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
}

type mapRequestStore interface {
	ListRequests(context.Context) ([]mapRequestRecord, error)
	CreateRequest(context.Context, mapRequestRecord) error
	JoinRequest(context.Context, string, string) (mapRequestRecord, error)
}

func publicRequest(record mapRequestRecord, accountID string) publicMapRequest {
	joined := false
	for _, participantID := range record.Participants {
		if participantID == accountID {
			joined = true
			break
		}
	}
	return publicMapRequest{
		ID: record.ID, CreatorName: record.CreatorName,
		CreatorInterests: record.CreatorInterests, Description: record.Description,
		Lat: record.Lat, Lng: record.Lng, CreatedAt: record.CreatedAt,
		ParticipantCount: len(record.Participants), JoinedByMe: joined,
		IsCreator: record.CreatorID == accountID,
	}
}

func newMapRequestID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func registerMapRequestRoutes(mux *http.ServeMux, store mapRequestStore, accounts accountStore) {
	mux.HandleFunc("GET /api/requests", func(w http.ResponseWriter, r *http.Request) {
		account, _, err := accountForRequest(r.Context(), r, accounts)
		if err != nil {
			writeAccountError(w, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		records, err := store.ListRequests(ctx)
		if err != nil {
			log.Printf("list map requests: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "solicitudes no disponibles"})
			return
		}
		result := make([]publicMapRequest, 0, len(records))
		for _, record := range records {
			result = append(result, publicRequest(record, account.ID))
		}
		jsonResponse(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/requests", func(w http.ResponseWriter, r *http.Request) {
		account, _, err := accountForRequest(r.Context(), r, accounts)
		if err != nil {
			writeAccountError(w, err)
			return
		}
		if !account.QuizComplete {
			jsonResponse(w, http.StatusForbidden, map[string]string{"error": "completa tu perfil antes de publicar una solicitud"})
			return
		}
		var input createMapRequestInput
		if err := decodeJSON(w, r, &input); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "solicitud no válida"})
			return
		}
		description := strings.TrimSpace(input.Description)
		if len([]rune(description)) < 10 || len([]rune(description)) > 500 ||
			math.IsNaN(input.Lat) || math.IsInf(input.Lat, 0) || input.Lat < -90 || input.Lat > 90 ||
			math.IsNaN(input.Lng) || math.IsInf(input.Lng, 0) || input.Lng < -180 || input.Lng > 180 {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "escribe una descripción de 10 a 500 caracteres y elige una ubicación válida"})
			return
		}
		id, err := newMapRequestID()
		if err != nil {
			log.Printf("generate map request id: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo crear la solicitud"})
			return
		}
		record := mapRequestRecord{
			ID: id, CreatorID: account.ID, CreatorName: account.Name,
			CreatorInterests: append([]string(nil), account.Interests...),
			Description:      description, Lat: math.Round(input.Lat*1000) / 1000,
			Lng:          math.Round(input.Lng*1000) / 1000,
			Participants: []string{account.ID}, CreatedAt: time.Now().UTC(),
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := store.CreateRequest(ctx, record); err != nil {
			log.Printf("create map request: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo guardar la solicitud"})
			return
		}
		jsonResponse(w, http.StatusCreated, publicRequest(record, account.ID))
	})

	mux.HandleFunc("POST /api/requests/{id}/join", func(w http.ResponseWriter, r *http.Request) {
		account, _, err := accountForRequest(r.Context(), r, accounts)
		if err != nil {
			writeAccountError(w, err)
			return
		}
		id := strings.TrimSpace(r.PathValue("id"))
		if len(id) != 32 {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "identificador de solicitud no válido"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		record, err := store.JoinRequest(ctx, id, account.ID)
		switch {
		case errors.Is(err, errRequestNotFound):
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "no se encontró la solicitud"})
		case errors.Is(err, errOwnRequest):
			jsonResponse(w, http.StatusConflict, map[string]string{"error": "no puedes apuntarte a tu propia solicitud"})
		case errors.Is(err, errAlreadyJoined):
			jsonResponse(w, http.StatusConflict, map[string]string{"error": "ya te apuntaste a esta solicitud"})
		case err != nil:
			log.Printf("join map request: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo completar la inscripción"})
		default:
			jsonResponse(w, http.StatusOK, publicRequest(record, account.ID))
		}
	})
}
