package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func routes(store requestStore, accounts accountStore) http.Handler {
	return routesWithPlaces(store, accounts, newPlacesClient(nil))
}

func routesWithPlaces(store requestStore, accounts accountStore, places *placesClient) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/places", places.handler(accounts))
	registerAuthRoutes(mux, accounts)
	registerMapRequestRoutes(mux, store, accounts)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if checker, ok := store.(interface{ Ping(context.Context) error }); ok {
			if err := checker.Ping(ctx); err != nil {
				jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
				return
			}
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func main() {
	if err := loadEnvFile(); err != nil {
		log.Fatal(err)
	}
	cfg, err := readConfig()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	client, store, err := connectMongo(ctx, cfg.MongoURI, cfg.Database)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	accounts := accountStore(store)
	log.Printf("using MongoDB database %s", cfg.Database)
	handler := routes(store, accounts)
	if cfg.WebDir != "" {
		if info, err := os.Stat(filepath.Join(cfg.WebDir, "index.html")); err != nil || info.IsDir() {
			log.Fatal("WEB_DIR must contain the Angular build index.html")
		}
		handler = withWeb(handler, cfg.WebDir)
		log.Printf("serving Angular from %s", cfg.WebDir)
	}
	server := &http.Server{Addr: cfg.Addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Go API listening on http://%s", cfg.Addr)
	log.Fatal(server.ListenAndServe())
}
