package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jellybrarian/ffmpeg"
)

func TestServeMovieClip(t *testing.T) {
	cfg := testConfig(t)
	movies := filepath.Join(filepath.Dir(cfg.Media), "movies")
	if err := os.MkdirAll(filepath.Join(movies, "Example"), 0755); err != nil {
		t.Fatalf("create movie dir: %v", err)
	}
	cfg.JellyfinMovies = []string{movies}
	video := filepath.Join(movies, "Example", "Example.mkv")
	if err := os.WriteFile(video, []byte("video"), 0644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	setupServerClipTools(t, false)

	query := url.Values{"path": {video}, "start": {"00:01:30.500"}, "duration": {"30"}}
	req := httptest.NewRequest(http.MethodGet, "/media/movies/clip?"+query.Encode(), nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()
	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
	}
	if resp.Body.String() != "generated clip" {
		t.Fatalf("unexpected clip: %q", resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); got != "video/mp4" {
		t.Fatalf("unexpected content type: %q", got)
	}
	if !strings.Contains(resp.Header().Get("Content-Disposition"), "Example-clip.mp4") {
		t.Fatalf("unexpected disposition: %q", resp.Header().Get("Content-Disposition"))
	}
	entries, err := os.ReadDir(ffmpeg.ClipTempDir())
	if err != nil {
		t.Fatalf("read clip temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected transferred clip to be removed, found %d entries", len(entries))
	}
}

func TestServeClipRemovesFileAfterClientWriteFailure(t *testing.T) {
	cfg := testConfig(t)
	movies := filepath.Join(filepath.Dir(cfg.Media), "movies")
	if err := os.MkdirAll(movies, 0755); err != nil {
		t.Fatalf("create movie dir: %v", err)
	}
	cfg.JellyfinMovies = []string{movies}
	video := filepath.Join(movies, "movie.mkv")
	if err := os.WriteFile(video, []byte("video"), 0644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	setupServerClipTools(t, false)

	query := url.Values{"path": {video}, "start": {"0"}, "duration": {"30"}}
	req := httptest.NewRequest(http.MethodGet, "/media/movies/clip?"+query.Encode(), nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	w := &failingResponseWriter{header: make(http.Header)}
	New(cfg).ServeHTTP(w, req)
	if w.writes == 0 {
		t.Fatal("expected response transfer to be attempted")
	}
	entries, err := os.ReadDir(ffmpeg.ClipTempDir())
	if err != nil {
		t.Fatalf("read clip temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected clip removal after transfer failure, found %d entries", len(entries))
	}
}

func TestServeTVClipReturnsInsufficientStorage(t *testing.T) {
	cfg := testConfig(t)
	tv := filepath.Join(filepath.Dir(cfg.Media), "tv")
	if err := os.MkdirAll(filepath.Join(tv, "Example", "Season 1"), 0755); err != nil {
		t.Fatalf("create TV dir: %v", err)
	}
	cfg.JellyfinTV = []string{tv}
	video := filepath.Join(tv, "Example", "Season 1", "episode.mkv")
	if err := os.WriteFile(video, []byte("video"), 0644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	setupServerClipTools(t, true)

	query := url.Values{"path": {video}, "start": {"0"}, "duration": {"30"}}
	req := httptest.NewRequest(http.MethodGet, "/media/tv/clip?"+query.Encode(), nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()
	New(cfg).ServeHTTP(resp, req)

	if resp.Code != http.StatusInsufficientStorage {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInsufficientStorage, resp.Code, resp.Body.String())
	}
	entries, err := os.ReadDir(ffmpeg.ClipTempDir())
	if err != nil {
		t.Fatalf("read clip temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected partial clip removal, found %d entries", len(entries))
	}
}

func TestServeClipRejectsOutsideSubtitle(t *testing.T) {
	cfg := testConfig(t)
	movies := filepath.Join(filepath.Dir(cfg.Media), "movies")
	if err := os.MkdirAll(movies, 0755); err != nil {
		t.Fatalf("create movies dir: %v", err)
	}
	cfg.JellyfinMovies = []string{movies}
	video := filepath.Join(movies, "movie.mkv")
	outside := filepath.Join(filepath.Dir(movies), "outside.srt")
	for _, path := range []string{video, outside} {
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	query := url.Values{"path": {video}, "start": {"0"}, "duration": {"30"}, "subtitle-path": {outside}}
	req := httptest.NewRequest(http.MethodGet, "/media/movies/clip?"+query.Encode(), nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()
	New(cfg).ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func TestParseClipTime(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
	}{
		{value: "30.5", want: 30*time.Second + 500*time.Millisecond},
		{value: "01:02:03.250", want: time.Hour + 2*time.Minute + 3*time.Second + 250*time.Millisecond},
	}
	for _, tt := range tests {
		got, err := parseClipTime(tt.value, "start")
		if err != nil {
			t.Fatalf("parseClipTime(%q): %v", tt.value, err)
		}
		if got != tt.want {
			t.Fatalf("parseClipTime(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
	for _, value := range []string{"", "1:02", "00:60:00", "00:00:60", "NaN"} {
		if _, err := parseClipTime(value, "start"); err == nil {
			t.Errorf("expected parseClipTime(%q) to fail", value)
		}
	}
}

func TestServeClipRejectsSubMillisecondDuration(t *testing.T) {
	cfg := testConfig(t)
	movies := filepath.Join(filepath.Dir(cfg.Media), "movies")
	if err := os.MkdirAll(movies, 0755); err != nil {
		t.Fatalf("create movies dir: %v", err)
	}
	cfg.JellyfinMovies = []string{movies}
	video := filepath.Join(movies, "movie.mkv")
	if err := os.WriteFile(video, []byte("video"), 0644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	query := url.Values{"path": {video}, "start": {"0"}, "duration": {"0.0001"}}
	req := httptest.NewRequest(http.MethodGet, "/media/movies/clip?"+query.Encode(), nil)
	req.Header.Set("X-Jellybrarian-Token", "test-secret")
	resp := httptest.NewRecorder()
	New(cfg).ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body.String())
	}
}

func setupServerClipTools(t *testing.T, noSpace bool) {
	t.Helper()
	binDir := t.TempDir()
	probe := `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_name":"h264","codec_type":"video"},{"index":1,"codec_name":"aac","codec_type":"audio"}]}'
`
	if err := os.WriteFile(filepath.Join(binDir, "ffprobe"), []byte(probe), 0755); err != nil {
		t.Fatalf("write ffprobe: %v", err)
	}
	command := "#!/bin/sh\nfor last; do :; done\nprintf '%s' 'generated clip' > \"$last\"\n"
	if noSpace {
		command += "printf '%s\\n' 'No space left on device' >&2\nexit 1\n"
	}
	if err := os.WriteFile(filepath.Join(binDir, "ffmpeg"), []byte(command), 0755); err != nil {
		t.Fatalf("write ffmpeg: %v", err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("TMPDIR", t.TempDir())
}

type failingResponseWriter struct {
	header http.Header
	writes int
}

func (w *failingResponseWriter) Header() http.Header {
	return w.header
}

func (w *failingResponseWriter) WriteHeader(int) {}

func (w *failingResponseWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, errors.New("client disconnected")
}
