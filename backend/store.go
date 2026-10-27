package main

import (
	"context"
	"errors"
	"strings"
)

var errNotFound = errors.New("profile not found")

type profileStore interface {
	List(context.Context, string, string) ([]Profile, error)
	ByID(context.Context, string) (Profile, error)
}

type demoStore struct{}

func (demoStore) List(_ context.Context, interest, search string) ([]Profile, error) {
	matches := make([]Profile, 0, len(profiles))
	search = strings.ToLower(search)
	for _, p := range profiles {
		if interest != "" && interest != "Everyone" {
			found := false
			for _, item := range p.Interests {
				if strings.EqualFold(item, interest) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if search != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.Role+" "+strings.Join(p.Interests, " ")), search) {
			continue
		}
		matches = append(matches, p)
	}
	return matches, nil
}

func (demoStore) ByID(_ context.Context, id string) (Profile, error) {
	for _, p := range profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return Profile{}, errNotFound
}
