package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const placesURL = "https://nominatim.openstreetmap.org/search"

type nearbyPlace struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Category       string  `json:"category"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	DistanceMeters int     `json:"distanceMeters"`
}

type nominatimPlace struct {
	ID   int64  `json:"place_id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Lat  string `json:"lat"`
	Lon  string `json:"lon"`
}

type placesClient struct{ http *http.Client }

func newPlacesClient(client *http.Client, _ ...string) *placesClient {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &placesClient{http: client}
}

func (c *placesClient) search(ctx context.Context, lat, lng float64) ([]nearbyPlace, error) {
	const radiusMeters = 1200.0
	deltaLat := radiusMeters / 111320
	deltaLng := radiusMeters / (111320 * math.Max(math.Abs(math.Cos(lat*math.Pi/180)), 0.1))
	params := url.Values{
		"q":       {"[cafe]"},
		"format":  {"jsonv2"},
		"limit":   {"40"},
		"bounded": {"1"},
		"viewbox": {fmt.Sprintf("%.6f,%.6f,%.6f,%.6f", lng-deltaLng, lat+deltaLat, lng+deltaLng, lat-deltaLat)},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, placesURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "SinglesFinder/1.0 (nearby cafes feature)")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Nominatim returned HTTP %d", response.StatusCode)
	}
	var results []nominatimPlace
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&results); err != nil {
		return nil, err
	}
	places := make([]nearbyPlace, 0, len(results))
	for _, result := range results {
		placeLat, latErr := strconv.ParseFloat(result.Lat, 64)
		placeLng, lngErr := strconv.ParseFloat(result.Lon, 64)
		if result.ID <= 0 || latErr != nil || lngErr != nil || math.IsNaN(placeLat) || math.IsInf(placeLat, 0) || math.IsNaN(placeLng) || math.IsInf(placeLng, 0) || placeLat < -90 || placeLat > 90 || placeLng < -180 || placeLng > 180 {
			continue
		}
		name := strings.TrimSpace(result.Name)
		if name == "" {
			name = "Cafetería cercana"
		}
		places = append(places, nearbyPlace{ID: strconv.FormatInt(result.ID, 10), Name: name, Category: "cafe", Lat: placeLat, Lng: placeLng, DistanceMeters: int(math.Round(distanceMeters(lat, lng, placeLat, placeLng)))})
	}
	return places, nil
}

func distanceMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusMeters = 6371000.0
	toRadians := math.Pi / 180
	deltaLat := (lat2 - lat1) * toRadians
	deltaLng := (lng2 - lng1) * toRadians
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) + math.Cos(lat1*toRadians)*math.Cos(lat2*toRadians)*math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	return 2 * earthRadiusMeters * math.Asin(math.Sqrt(a))
}

func (c *placesClient) handler(accounts accountStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if accounts == nil {
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		if _, err := requireAccount(w, r, accounts); err != nil {
			return
		}
		query := r.URL.Query()
		lat, latErr := strconv.ParseFloat(query.Get("lat"), 64)
		lng, lngErr := strconv.ParseFloat(query.Get("lng"), 64)
		if latErr != nil || lngErr != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || math.IsNaN(lng) || math.IsInf(lng, 0) || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "lat and lng must be valid coordinates"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		places, err := c.search(ctx, lat, lng)
		if err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				log.Printf("nearby places provider failed: %v", err)
			}
			jsonResponse(w, http.StatusBadGateway, map[string]string{"error": "nearby places are temporarily unavailable"})
			return
		}
		jsonResponse(w, http.StatusOK, places)
	}
}
