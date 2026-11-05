package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type placesRoundTripper func(*http.Request) (*http.Response, error)

func (f placesRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func placesResponse(status int, body string, request *http.Request) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}
}

func TestPlacesRequiresAuthenticationAndValidCoordinates(t *testing.T) {
	called := false
	client := newPlacesClient(&http.Client{Transport: placesRoundTripper(func(r *http.Request) (*http.Response, error) {
		called = true
		return placesResponse(http.StatusOK, `[]`, r), nil
	})})
	unauthorized := httptest.NewRecorder()
	client.handler(nil).ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/places?lat=0&lng=0", nil))
	if unauthorized.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d upstream-called=%v", unauthorized.Code, called)
	}
	accounts := newMemoryAccountStore()
	cookie := testSession(t, accounts, Account{ID: "u1", Email: "local@example.test", Status: "active"})
	invalid := httptest.NewRequest(http.MethodGet, "/api/places?lat=91&lng=0", nil)
	invalid.AddCookie(cookie)
	response := httptest.NewRecorder()
	client.handler(accounts).ServeHTTP(response, invalid)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "valid coordinates") || called {
		t.Fatalf("invalid response: %d %s", response.Code, response.Body.String())
	}
}

func TestPlacesReadsNominatimAndHidesProviderError(t *testing.T) {
	accounts := newMemoryAccountStore()
	cookie := testSession(t, accounts, Account{ID: "u1", Email: "local@example.test", Status: "active"})
	transport := placesRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Host != "nominatim.openstreetmap.org" || r.Header.Get("User-Agent") == "" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if r.URL.Query().Get("q") != "[cafe]" || r.URL.Query().Get("limit") != "40" || r.URL.Query().Get("bounded") != "1" {
			t.Errorf("unexpected Nominatim query: %s", r.URL.RawQuery)
		}
		return placesResponse(http.StatusOK, `[{"place_id":5,"lat":"28.6","lon":"-106.1","name":"Cafe Uno","type":"cafe"}]`, r), nil
	})
	client := newPlacesClient(&http.Client{Transport: transport})
	request := httptest.NewRequest(http.MethodGet, "/api/places?lat=28.6&lng=-106.1", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	client.handler(accounts).ServeHTTP(response, request)
	var places []nearbyPlace
	if err := json.Unmarshal(response.Body.Bytes(), &places); err != nil || response.Code != http.StatusOK || len(places) != 1 || places[0].Name != "Cafe Uno" {
		t.Fatalf("status=%d places=%+v body=%s err=%v", response.Code, places, response.Body.String(), err)
	}
	failure := newPlacesClient(&http.Client{Transport: placesRoundTripper(func(r *http.Request) (*http.Response, error) {
		return placesResponse(http.StatusServiceUnavailable, "private provider details", r), nil
	})})
	request = httptest.NewRequest(http.MethodGet, "/api/places?lat=28.6&lng=-106.1", nil)
	request.AddCookie(cookie)
	response = httptest.NewRecorder()
	failure.handler(accounts).ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway || strings.Contains(response.Body.String(), "private provider") {
		t.Fatalf("provider error leaked: %d %s", response.Code, response.Body.String())
	}
}
