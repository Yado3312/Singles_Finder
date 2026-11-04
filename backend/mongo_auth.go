package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (s mongoStore) CreateAccount(ctx context.Context, account Account) error {
	_, err := s.database.Collection("accounts").InsertOne(ctx, account)
	if mongo.IsDuplicateKeyError(err) {
		return errEmailTaken
	}
	if err != nil {
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

func (s mongoStore) FindByEmail(ctx context.Context, email string) (Account, error) {
	var account Account
	err := s.database.Collection("accounts").FindOne(ctx, bson.D{{Key: "email", Value: email}}).Decode(&account)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Account{}, errInvalidCredentials
	}
	if err != nil {
		return Account{}, fmt.Errorf("find account: %w", err)
	}
	return account, nil
}

func (s mongoStore) GetAccount(ctx context.Context, id string) (Account, error) {
	var account Account
	err := s.database.Collection("accounts").FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&account)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Account{}, errInvalidSession
	}
	if err != nil {
		return Account{}, fmt.Errorf("get account: %w", err)
	}
	return account, nil
}

func (s mongoStore) UpdateQuiz(ctx context.Context, id string, input quizRequest) error {
	result, err := s.database.Collection("accounts").UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "status", Value: "active"}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "interests", Value: input.Interests},
			{Key: "personality", Value: input.Personality},
			{Key: "datePace", Value: input.DatePace},
			{Key: "quizComplete", Value: true},
			{Key: "updatedAt", Value: time.Now().UTC()},
		}}},
	)
	if err != nil {
		return fmt.Errorf("update personality test: %w", err)
	}
	if result.MatchedCount == 0 {
		return errInvalidSession
	}
	return nil
}

func (s mongoStore) UpdateName(ctx context.Context, id, name string) error {
	result, err := s.database.Collection("accounts").UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "status", Value: "active"}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "name", Value: name}, {Key: "updatedAt", Value: time.Now().UTC()}}}},
	)
	if err != nil {
		return fmt.Errorf("update account name: %w", err)
	}
	if result.MatchedCount == 0 {
		return errInvalidSession
	}
	return nil
}

func (s mongoStore) CreateSession(ctx context.Context, session authSession) error {
	_, err := s.database.Collection("sessions").InsertOne(ctx, session)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s mongoStore) GetSession(ctx context.Context, tokenHash string) (authSession, error) {
	var session authSession
	err := s.database.Collection("sessions").FindOne(ctx, bson.D{{Key: "_id", Value: tokenHash}}).Decode(&session)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return authSession{}, errInvalidSession
	}
	if err != nil {
		return authSession{}, fmt.Errorf("get session: %w", err)
	}
	return session, nil
}

func (s mongoStore) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.database.Collection("sessions").DeleteOne(ctx, bson.D{{Key: "_id", Value: tokenHash}}); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
