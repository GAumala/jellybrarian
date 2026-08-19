package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"jellybrarian/config"
)

func TestAuthRequired(t *testing.T) {
	handler := New(testConfig(t))

	req := httptest.NewRequest(http.MethodGet, "/media/list", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.Code)
	}
}

func TestAuthWithTokenHeader(t *testing.T) {
	handler := New(testConfig(t))

	req := httptest.NewRequest(http.MethodGet, "/media/list", nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
}

func TestAuthWithBearerToken(t *testing.T) {
	handler := New(testConfig(t))

	req := httptest.NewRequest(http.MethodGet, "/media/list", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
}

func TestAuthDisabledWithoutConfiguredToken(t *testing.T) {
	cfg := testConfig(t)
	cfg.AuthToken = ""
	handler := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/media/list", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
}

func TestDelistArtistEndpoint(t *testing.T) {
	cfg := testConfig(t)
	artistDir := filepath.Join(cfg.JellyfinMusic[0], "Radiohead")
	if err := os.MkdirAll(filepath.Join(artistDir, "Kid A"), 0755); err != nil {
		t.Fatalf("failed to create artist dir: %v", err)
	}
	handler := New(cfg)

	req := httptest.NewRequest(http.MethodPut, "/media/artists/Radiohead/delist", nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}
	if _, err := os.Stat(artistDir); !os.IsNotExist(err) {
		t.Fatal("expected artist library folder removed")
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	mediaDir := filepath.Join(root, "media")
	musicDir := filepath.Join(root, "music")
	for _, dir := range []string{mediaDir, musicDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	return &config.Config{
		Media:         mediaDir,
		AuthToken:     "test-secret",
		JellyfinMusic: []string{musicDir},
	}
}
