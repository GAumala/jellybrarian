package ffmpeg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCreateClipCopiesVideoAndAAC(t *testing.T) {
	binDir := setupClipTools(t, `{"streams":[{"index":0,"codec_name":"h264","codec_type":"video"},{"index":2,"codec_name":"aac","codec_type":"audio"}]}`, false)
	t.Setenv("PATH", binDir)
	t.Setenv("TMPDIR", t.TempDir())

	output, err := CreateClip(context.Background(), "/library/movie.mkv", ClipOptions{
		Start:    90 * time.Second,
		Duration: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("CreateClip failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(output) })
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "clip data" {
		t.Fatalf("unexpected output: %q", data)
	}

	args, err := os.ReadFile(filepath.Join(binDir, "args"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	for _, expected := range []string{
		"-ss\n90.000\n", "-t\n30.000\n", "-map\n0:0\n", "-map\n0:2\n",
		"-c:v\ncopy\n", "-c:a\ncopy\n", "-movflags\n+faststart\n",
	} {
		if !strings.Contains(string(args), expected) {
			t.Errorf("ffmpeg args missing %q:\n%s", expected, args)
		}
	}
}

func TestCreateClipBurnsSelectedSubtitleAndConvertsAudio(t *testing.T) {
	binDir := setupClipTools(t, `{"streams":[{"index":0,"codec_name":"h264","codec_type":"video"},{"index":1,"codec_name":"subrip","codec_type":"subtitle"},{"index":3,"codec_name":"ac3","codec_type":"audio"},{"index":4,"codec_name":"ass","codec_type":"subtitle"}]}`, false)
	t.Setenv("PATH", binDir)
	t.Setenv("TMPDIR", t.TempDir())
	subtitle := 4

	output, err := CreateClip(context.Background(), "/library/movie.mkv", ClipOptions{
		Start:          5 * time.Second,
		Duration:       10 * time.Second,
		SubtitleStream: &subtitle,
	})
	if err != nil {
		t.Fatalf("CreateClip failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(output) })
	args, err := os.ReadFile(filepath.Join(binDir, "args"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	for _, expected := range []string{
		"subtitles=filename='/library/movie.mkv':si=1", "-c:v\nlibx264\n",
		"-c:a\naac\n", "-b:a\n192k\n",
	} {
		if !strings.Contains(string(args), expected) {
			t.Errorf("ffmpeg args missing %q:\n%s", expected, args)
		}
	}
}

func TestClipArgsEncodesAACWhenBurningSubtitles(t *testing.T) {
	options := ClipOptions{Duration: time.Second, SubtitlePath: "/library/movie.srt"}
	streams := clipStreams{
		video: probeStream{Index: 0, CodecName: "h264", CodecType: "video"},
		audio: &probeStream{Index: 1, CodecName: "aac", CodecType: "audio"},
	}
	args := strings.Join(clipArgs("/library/movie.mkv", "/tmp/clip.mp4", options, streams), "\n")
	if !strings.Contains(args, "-c:a\naac") {
		t.Fatalf("expected subtitle mode to encode AAC audio:\n%s", args)
	}
}

func TestCreateClipRejectsWrongStreamType(t *testing.T) {
	binDir := setupClipTools(t, `{"streams":[{"index":0,"codec_name":"h264","codec_type":"video"},{"index":1,"codec_name":"aac","codec_type":"audio"}]}`, false)
	t.Setenv("PATH", binDir)
	t.Setenv("TMPDIR", t.TempDir())
	audio := 0

	_, err := CreateClip(context.Background(), "/library/movie.mkv", ClipOptions{Duration: time.Second, AudioStream: &audio})
	if !errors.Is(err, ErrInvalidStream) {
		t.Fatalf("expected ErrInvalidStream, got %v", err)
	}
}

func TestCreateClipRemovesPartialFileOnNoSpace(t *testing.T) {
	binDir := setupClipTools(t, `{"streams":[{"index":0,"codec_name":"h264","codec_type":"video"}]}`, true)
	t.Setenv("PATH", binDir)
	t.Setenv("TMPDIR", t.TempDir())

	_, err := CreateClip(context.Background(), "/library/movie.mkv", ClipOptions{Duration: time.Second})
	if !errors.Is(err, ErrNoSpace) {
		t.Fatalf("expected ErrNoSpace, got %v", err)
	}
	entries, readErr := os.ReadDir(ClipTempDir())
	if readErr != nil {
		t.Fatalf("read clip temp dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected partial clip removal, found %d entries", len(entries))
	}
}

func TestIsNoSpaceRecognizesQuotaExhaustion(t *testing.T) {
	if !isNoSpace(syscall.EDQUOT, "") {
		t.Fatal("expected EDQUOT to be treated as insufficient storage")
	}
	if !isNoSpace(errors.New("ffmpeg failed"), "Disk quota exceeded") {
		t.Fatal("expected quota diagnostic to be treated as insufficient storage")
	}
}

func TestCreateClipRemovesPartialFileOnCancellation(t *testing.T) {
	binDir := t.TempDir()
	probe := `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_name":"h264","codec_type":"video"}]}'
`
	if err := os.WriteFile(filepath.Join(binDir, "ffprobe"), []byte(probe), 0755); err != nil {
		t.Fatalf("write ffprobe: %v", err)
	}
	started := filepath.Join(binDir, "started")
	command := "#!/bin/sh\nfor last; do :; done\nprintf '%s' 'partial' > \"$last\"\nprintf '%s' started > '" + started + "'\nwhile :; do :; done\n"
	if err := os.WriteFile(filepath.Join(binDir, "ffmpeg"), []byte(command), 0755); err != nil {
		t.Fatalf("write ffmpeg: %v", err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("TMPDIR", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := CreateClip(ctx, "/library/movie.mkv", ClipOptions{Duration: time.Second})
		result <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ffmpeg did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CreateClip did not stop after cancellation")
	}
	entries, err := os.ReadDir(ClipTempDir())
	if err != nil {
		t.Fatalf("read clip temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected cancelled clip removal, found %d entries", len(entries))
	}
}

func TestCleanupClipTempDirOnlyRemovesClipFiles(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dir, err := ensureClipTempDir()
	if err != nil {
		t.Fatalf("ensure temp dir: %v", err)
	}
	stale := filepath.Join(dir, "clip-stale.mp4")
	unrelated := filepath.Join(dir, "keep.txt")
	for _, path := range []string{stale, unrelated} {
		if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := CleanupClipTempDir(); err != nil {
		t.Fatalf("CleanupClipTempDir failed: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale clip removed, got %v", err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("expected unrelated file preserved: %v", err)
	}
}

func TestCleanupClipTempDirRejectsSymlink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	target := t.TempDir()
	stale := filepath.Join(target, "clip-stale.mp4")
	if err := os.WriteFile(stale, []byte("data"), 0600); err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	if err := os.Symlink(target, ClipTempDir()); err != nil {
		t.Fatalf("create temp-dir symlink: %v", err)
	}
	if err := CleanupClipTempDir(); err == nil {
		t.Fatal("expected symlinked clip directory to be rejected")
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("expected target file to remain: %v", err)
	}
}

func setupClipTools(t *testing.T, probeJSON string, noSpace bool) string {
	t.Helper()
	binDir := t.TempDir()
	probeScript := "#!/bin/sh\nprintf '%s' '" + probeJSON + "'\n"
	if err := os.WriteFile(filepath.Join(binDir, "ffprobe"), []byte(probeScript), 0755); err != nil {
		t.Fatalf("write ffprobe: %v", err)
	}
	ffmpegScript := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + filepath.Join(binDir, "args") + "'\nfor last; do :; done\nprintf '%s' 'clip data' > \"$last\"\n"
	if noSpace {
		ffmpegScript += "printf '%s\\n' 'No space left on device' >&2\nexit 1\n"
	}
	if err := os.WriteFile(filepath.Join(binDir, "ffmpeg"), []byte(ffmpegScript), 0755); err != nil {
		t.Fatalf("write ffmpeg: %v", err)
	}
	return binDir
}
