package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

func TestServeScopedFile(t *testing.T) {
	cfg := testConfig(t)
	for _, dir := range []string{filepath.Join(filepath.Dir(cfg.Media), "movies"), filepath.Join(filepath.Dir(cfg.Media), "tv")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create library dir: %v", err)
		}
	}
	cfg.JellyfinMovies = []string{filepath.Join(filepath.Dir(cfg.Media), "movies")}
	cfg.JellyfinTV = []string{filepath.Join(filepath.Dir(cfg.Media), "tv")}

	tests := []struct {
		name  string
		route string
		root  string
	}{
		{name: "media", route: "/media/file", root: cfg.Media},
		{name: "movies", route: "/media/movies/file", root: cfg.JellyfinMovies[0]},
		{name: "tv", route: "/media/tv/file", root: cfg.JellyfinTV[0]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tt.root, "inspect.txt")
			if err := os.WriteFile(path, []byte("inspect me"), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, tt.route+"?path="+url.QueryEscape(path), nil)
			req.Header.Set("X-Jellybrarian-Token", "test-secret")
			resp := httptest.NewRecorder()

			New(cfg).ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response: %v", err)
			}
			if string(body) != "inspect me" {
				t.Fatalf("expected file contents, got %q", body)
			}
		})
	}
}

func TestServeScopedFileRejectsOutsidePath(t *testing.T) {
	cfg := testConfig(t)
	outside := filepath.Join(filepath.Dir(cfg.Media), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to write outside file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/media/file?path="+url.QueryEscape(outside), nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.Code)
	}
	if strings.Contains(resp.Body.String(), "secret") {
		t.Fatal("outside file contents were served")
	}
}

func TestUploadScopedFile(t *testing.T) {
	cfg := testConfig(t)
	targetDir := filepath.Join(cfg.Media, "uploads")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatalf("failed to create upload directory: %v", err)
	}
	target := filepath.Join(targetDir, "movie.mkv")
	req := httptest.NewRequest(http.MethodPut, "/media/file?path="+url.QueryEscape(target), strings.NewReader("video data"))
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, resp.Code, resp.Body.String())
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read uploaded file: %v", err)
	}
	if string(contents) != "video data" {
		t.Fatalf("expected uploaded contents, got %q", contents)
	}
}

func TestUploadScopedFileRejectsOutsidePath(t *testing.T) {
	cfg := testConfig(t)
	target := filepath.Join(filepath.Dir(cfg.Media), "outside.mkv")
	req := httptest.NewRequest(http.MethodPut, "/media/file?path="+url.QueryEscape(target), strings.NewReader("video data"))
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("expected outside target not to be created")
	}
}

func TestUploadScopedFileDoesNotOverwrite(t *testing.T) {
	cfg := testConfig(t)
	target := filepath.Join(cfg.Media, "existing.mkv")
	if err := os.WriteFile(target, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/media/file?path="+url.QueryEscape(target), strings.NewReader("replacement"))
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, resp.Code, resp.Body.String())
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read existing file: %v", err)
	}
	if string(contents) != "original" {
		t.Fatalf("existing file was overwritten with %q", contents)
	}
}

func TestUploadScopedFileRejectsSymlinkOutsidePath(t *testing.T) {
	cfg := testConfig(t)
	outside := t.TempDir()
	link := filepath.Join(cfg.Media, "outside-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}
	target := filepath.Join(link, "escaped.mkv")
	req := httptest.NewRequest(http.MethodPut, "/media/file?path="+url.QueryEscape(target), strings.NewReader("video data"))
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
	if _, err := os.Stat(filepath.Join(outside, "escaped.mkv")); !os.IsNotExist(err) {
		t.Fatal("expected symlinked outside target not to be created")
	}
}

func TestServeFFProbe(t *testing.T) {
	cfg := testConfig(t)
	path := filepath.Join(cfg.Media, "inspect.mkv")
	if err := os.WriteFile(path, []byte("not a real media file"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	binDir := t.TempDir()
	ffprobe := filepath.Join(binDir, "ffprobe")
	script := "#!/bin/sh\nprintf '%s' '{\"streams\":[],\"format\":{\"filename\":\"inspect.mkv\"}}'\n"
	if err := os.WriteFile(ffprobe, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake ffprobe: %v", err)
	}
	t.Setenv("PATH", binDir)

	req := httptest.NewRequest(http.MethodGet, "/media/ffprobe?path="+url.QueryEscape(path), nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if got := resp.Body.String(); got != "{\"streams\":[],\"format\":{\"filename\":\"inspect.mkv\"}}" {
		t.Fatalf("unexpected ffprobe output: %q", got)
	}
}

func TestServeAudio(t *testing.T) {
	cfg := testConfig(t)
	path := filepath.Join(cfg.Media, "inspect.mkv")
	if err := os.WriteFile(path, []byte("video"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	binDir := t.TempDir()
	argsPath := filepath.Join(binDir, "args")
	ffmpeg := filepath.Join(binDir, "ffmpeg")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + argsPath + "'\nprintf '%s' 'audio data'\n"
	if err := os.WriteFile(ffmpeg, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake ffmpeg: %v", err)
	}
	t.Setenv("PATH", binDir)

	req := httptest.NewRequest(http.MethodGet, "/media/audio?path="+url.QueryEscape(path)+"&type=raw&stream=0%3A1&ext=aac", nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if resp.Body.String() != "audio data" {
		t.Fatalf("unexpected audio output: %q", resp.Body.String())
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("failed to read ffmpeg args: %v", err)
	}
	for _, expected := range []string{"-map\n0:1\n", "-c:a\ncopy\n", "-f\nadts\n", "pipe:1\n"} {
		if !strings.Contains(string(args), expected) {
			t.Errorf("ffmpeg args missing %q: %s", expected, args)
		}
	}
}

func TestServeAudioRequiresRawExtension(t *testing.T) {
	cfg := testConfig(t)
	path := filepath.Join(cfg.Media, "inspect.mkv")
	if err := os.WriteFile(path, []byte("video"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/media/audio?path="+url.QueryEscape(path)+"&type=raw&stream=0%3A1", nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.Code)
	}
}

func TestServeAudioRejectsUnknownRawExtension(t *testing.T) {
	cfg := testConfig(t)
	path := filepath.Join(cfg.Media, "inspect.mkv")
	if err := os.WriteFile(path, []byte("video"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/media/audio?path="+url.QueryEscape(path)+"&type=raw&stream=0%3A1&ext=mp3", nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()

	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.Code)
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
