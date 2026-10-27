package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Profile struct {
	ID        string   `json:"id" bson:"id"`
	Name      string   `json:"name" bson:"name"`
	Age       int      `json:"age" bson:"age"`
	Role      string   `json:"role" bson:"role"`
	Interests []string `json:"interests" bson:"interests"`
	Distance  string   `json:"distance" bson:"distance"`
	Lat       float64  `json:"lat" bson:"lat"`
	Lng       float64  `json:"lng" bson:"lng"`
	Color     string   `json:"color" bson:"color"`
	Initials  string   `json:"initials" bson:"initials"`
	Online    bool     `json:"online" bson:"online"`
	Bio       string   `json:"bio" bson:"bio"`
	Looking   string   `json:"looking" bson:"looking"`
	Image     string   `json:"image" bson:"image"`
}

// Fictional demo profiles. Coordinates are map display points, not live user locations.
var profiles = []Profile{
	{"demo-sofia", "Sofia", 27, "Designer & weekend explorer", []string{"Coffee", "Art", "Hiking"}, "1.2 km away", 28.6401, -106.0784, "#e6baa8", "S", true, "Usually looking for a new coffee spot, a good conversation, or somewhere with a view. Let’s start with your favorite local place.", "A meaningful connection", "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=500&q=85"},
	{"demo-daniel", "Daniel", 30, "Architect & dog person", []string{"Dogs", "Live music", "Food"}, "2.4 km away", 28.6298, -106.0651, "#d9b9a5", "D", true, "I believe great dates start with a walk, a playlist, and getting a little lost in a new neighborhood.", "Something real", "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?w=500&q=85"},
	{"demo-valeria", "Valeria", 26, "Book lover & curious mind", []string{"Books", "Travel", "Coffee"}, "3.1 km away", 28.6482, -106.1019, "#cbaeb3", "V", false, "Collecting little stories, big memories, and recommendations for books I will absolutely finish someday.", "A genuine relationship", "https://images.unsplash.com/photo-1524504388940-b1c1722653e1?w=500&q=85"},
	{"demo-alex", "Alex", 29, "Chef in the making", []string{"Cooking", "Art", "Travel"}, "4.6 km away", 28.6152, -106.0908, "#c8ad95", "A", true, "If you have a favorite recipe, I would love to hear the story behind it. Bonus points for spontaneous road trips.", "A meaningful connection", "https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?w=500&q=85"},
	{"demo-camila", "Camila", 28, "Music, mountains, and more", []string{"Hiking", "Live music", "Dogs"}, "5.2 km away", 28.6584, -106.0508, "#d3b5a2", "C", false, "Happiest outdoors or at a tiny concert. Looking to meet someone kind who enjoys both plans and surprises.", "Something real", "https://images.unsplash.com/photo-1531123897727-8f129e1688ce?w=500&q=85"},
	{"demo-mateo", "Mateo", 31, "Photographer & traveler", []string{"Travel", "Art", "Coffee"}, "6.8 km away", 28.6052, -106.0528, "#b8a99d", "M", true, "Always noticing the small details. I would love to explore the city with someone who sees the magic in ordinary days.", "A genuine relationship", "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=500&q=85"},
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func routes(store profileStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/profiles", func(w http.ResponseWriter, r *http.Request) {
		interest := strings.TrimSpace(r.URL.Query().Get("interest"))
		search := strings.TrimSpace(r.URL.Query().Get("search"))
		if len(interest) > 50 || len(search) > 100 {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "filter too long"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		matches, err := store.List(ctx, interest, search)
		if err != nil {
			log.Printf("list profiles: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "profiles unavailable"})
			return
		}
		jsonResponse(w, http.StatusOK, matches)
	})
	mux.HandleFunc("GET /api/profiles/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" || len(id) > 100 {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid profile id"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		p, err := store.ByID(ctx, id)
		if errors.Is(err, errNotFound) {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
			return
		}
		if err != nil {
			log.Printf("get profile: %v", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "profile unavailable"})
			return
		}
		jsonResponse(w, http.StatusOK, p)
	})
	return mux
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	var store profileStore = demoStore{}
	if uri := os.Getenv("MONGODB_URI"); uri != "" {
		database := os.Getenv("MONGODB_DATABASE")
		if database == "" {
			database = "singlesfinder"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		client, mongoProfiles, err := connectMongo(ctx, uri, database)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		defer client.Disconnect(context.Background())
		store = mongoProfiles
		if os.Getenv("SEED_DEMO") == "true" {
			ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
			err = mongoProfiles.(mongoStore).SeedDemo(ctx)
			cancel()
			if err != nil {
				log.Fatal(err)
			}
			log.Print("inserted fictional demo profiles")
		}
		log.Printf("using MongoDB database %s", database)
	} else {
		log.Print("MONGODB_URI is unset; using fictional in-memory profiles")
	}
	server := &http.Server{Addr: addr, Handler: routes(store), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Go API listening on http://%s", addr)
	log.Fatal(server.ListenAndServe())
}
