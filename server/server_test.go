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

func testConfig(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	mediaDir := filepath.Join(root, "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		t.Fatalf("failed to create media dir: %v", err)
	}

	return &config.Config{
		Media:     mediaDir,
		AuthToken: "test-secret",
	}
}
