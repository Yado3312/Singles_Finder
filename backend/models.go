package main

import "time"

// These documents are storage models for upcoming authenticated features.
// Do not return Account or PrivateLocation from public API handlers.
type Account struct {
	ID           string    `bson:"_id"`
	Email        string    `bson:"email"`
	PasswordHash string    `bson:"passwordHash"`
	BirthDate    time.Time `bson:"birthDate"`
	Status       string    `bson:"status"` // active, suspended, deleted
	CreatedAt    time.Time `bson:"createdAt"`
	UpdatedAt    time.Time `bson:"updatedAt"`
}

type GeoPoint struct {
	Type        string     `bson:"type"`        // Point
	Coordinates [2]float64 `bson:"coordinates"` // longitude, latitude
}

type PrivateLocation struct {
	AccountID      string    `bson:"accountId"`
	Point          GeoPoint  `bson:"point"`
	SharingEnabled bool      `bson:"sharingEnabled"`
	UpdatedAt      time.Time `bson:"updatedAt"`
	ExpiresAt      time.Time `bson:"expiresAt"`
}

type Like struct {
	FromAccountID string    `bson:"fromAccountId"`
	ToAccountID   string    `bson:"toAccountId"`
	CreatedAt     time.Time `bson:"createdAt"`
}

type Match struct {
	AccountIDs    [2]string  `bson:"accountIds"` // sorted once when created
	PairKey       string     `bson:"pairKey"`    // stable, unique, derived from sorted account IDs
	CreatedAt     time.Time  `bson:"createdAt"`
	LastMessageAt *time.Time `bson:"lastMessageAt,omitempty"`
}

type Message struct {
	MatchID         string     `bson:"matchId"`
	SenderAccountID string     `bson:"senderAccountId"`
	Body            string     `bson:"body"`
	CreatedAt       time.Time  `bson:"createdAt"`
	ReadAt          *time.Time `bson:"readAt,omitempty"`
}

type Block struct {
	FromAccountID string    `bson:"fromAccountId"`
	ToAccountID   string    `bson:"toAccountId"`
	CreatedAt     time.Time `bson:"createdAt"`
}

type Report struct {
	ReporterAccountID string    `bson:"reporterAccountId"`
	SubjectAccountID  string    `bson:"subjectAccountId"`
	Reason            string    `bson:"reason"`
	Details           string    `bson:"details,omitempty"`
	Status            string    `bson:"status"` // open, reviewing, resolved
	CreatedAt         time.Time `bson:"createdAt"`
}

// ProfileRecord adds ownership and timestamps to the public discovery projection.
// Public handlers decode only Profile, so accountId is never serialized to clients.
type ProfileRecord struct {
	Profile   `bson:",inline"`
	AccountID string    `bson:"accountId"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}
