package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebAndAPIShareOrigin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>Closer</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	handler := withWeb(routes(demoStore{}, nil), dir)
	for _, path := range []string{"/", "/discover"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Closer") {
			t.Fatalf("%s: %d %q", path, response.Code, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("API: %d %q", response.Code, response.Body.String())
	}
}
