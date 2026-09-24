package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const clipStderrLimit = 64 << 10

var (
	ErrInvalidStream     = errors.New("invalid media stream")
	ErrNoSpace           = errors.New("no space left for clip")
	ErrExecutableMissing = errors.New("ffmpeg or ffprobe executable not found")
)

type ClipOptions struct {
	Start          time.Duration
	Duration       time.Duration
	AudioStream    *int
	SubtitleStream *int
	SubtitlePath   string
}

type probeStream struct {
	Index       int    `json:"index"`
	CodecName   string `json:"codec_name"`
	CodecType   string `json:"codec_type"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

type probeResult struct {
	Streams []probeStream `json:"streams"`
}

type clipStreams struct {
	video           probeStream
	audio           *probeStream
	subtitleOrdinal int
}

// ClipTempDir is the private directory used for completed clips before transfer.
func ClipTempDir() string {
	return filepath.Join(os.TempDir(), "jellybrarian-clips")
}

func ensureClipTempDir() (string, error) {
	dir := ClipTempDir()
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		if isNoSpace(err, "") {
			return "", fmt.Errorf("%w: %v", ErrNoSpace, err)
		}
		return "", fmt.Errorf("create clip temp directory: %w", err)
	}
	if err := validateClipTempDir(dir); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		if isNoSpace(err, "") {
			return "", fmt.Errorf("%w: %v", ErrNoSpace, err)
		}
		return "", fmt.Errorf("secure clip temp directory: %w", err)
	}
	return dir, nil
}

// CleanupClipTempDir removes clip files left behind by an interrupted process.
func CleanupClipTempDir() error {
	dir := ClipTempDir()
	_, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect clip temp directory: %w", err)
	}
	if err := validateClipTempDir(dir); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read clip temp directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "clip-") || !strings.HasSuffix(entry.Name(), ".mp4") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale clip %q: %w", entry.Name(), err)
		}
	}
	return nil
}

func validateClipTempDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("inspect clip temp directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("clip temp path %q is not a real directory", dir)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("clip temp directory %q is not owned by the current user", dir)
	}
	return nil
}

// CreateClip writes a conventional MP4 to a temporary file. The caller owns
// the returned path and must remove it after serving it.
func CreateClip(ctx context.Context, path string, options ClipOptions) (string, error) {
	if options.Start < 0 || options.Duration <= 0 {
		return "", fmt.Errorf("start must be non-negative and duration must be positive")
	}
	if options.SubtitleStream != nil && options.SubtitlePath != "" {
		return "", fmt.Errorf("subtitle stream and subtitle path are mutually exclusive")
	}

	streams, err := inspectClipStreams(ctx, path, options)
	if err != nil {
		return "", err
	}
	dir, err := ensureClipTempDir()
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "clip-*.mp4")
	if err != nil {
		if isNoSpace(err, "") {
			return "", fmt.Errorf("%w: %v", ErrNoSpace, err)
		}
		return "", fmt.Errorf("create clip temp file: %w", err)
	}
	outputPath := tmp.Name()
	if closeErr := tmp.Close(); closeErr != nil {
		_ = os.Remove(outputPath)
		if isNoSpace(closeErr, "") {
			return "", fmt.Errorf("%w: %v", ErrNoSpace, closeErr)
		}
		return "", fmt.Errorf("close clip temp file: %w", closeErr)
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.Remove(outputPath)
		}
	}()

	args := clipArgs(path, outputPath, options, streams)
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrExecutableMissing, err)
	}
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	stderr := &boundedBuffer{limit: clipStderrLimit}
	cmd.Stderr = stderr
	if runErr := cmd.Run(); runErr != nil {
		message := strings.TrimSpace(stderr.String())
		if ctx.Err() != nil {
			return "", fmt.Errorf("ffmpeg canceled: %w", ctx.Err())
		}
		if isNoSpace(runErr, message) {
			return "", fmt.Errorf("%w: %s", ErrNoSpace, message)
		}
		if message != "" {
			return "", fmt.Errorf("ffmpeg failed: %w: %s", runErr, message)
		}
		return "", fmt.Errorf("ffmpeg failed: %w", runErr)
	}
	succeeded = true
	return outputPath, nil
}

func inspectClipStreams(ctx context.Context, path string, options ClipOptions) (clipStreams, error) {
	data, err := Probe(ctx, path)
	if err != nil {
		return clipStreams{}, err
	}
	var result probeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return clipStreams{}, fmt.Errorf("decode ffprobe output: %w", err)
	}

	var selected clipStreams
	foundVideo := false
	subtitleOrdinal := 0
	for i := range result.Streams {
		stream := result.Streams[i]
		switch stream.CodecType {
		case "video":
			if !foundVideo && stream.Disposition.AttachedPic == 0 {
				selected.video = stream
				foundVideo = true
			}
		case "audio":
			if options.AudioStream == nil && selected.audio == nil {
				copy := stream
				selected.audio = &copy
			}
			if options.AudioStream != nil && stream.Index == *options.AudioStream {
				copy := stream
				selected.audio = &copy
			}
		case "subtitle":
			if options.SubtitleStream != nil && stream.Index == *options.SubtitleStream {
				selected.subtitleOrdinal = subtitleOrdinal
			}
			subtitleOrdinal++
		}
	}
	if !foundVideo {
		return clipStreams{}, fmt.Errorf("%w: no video stream found", ErrInvalidStream)
	}
	if options.AudioStream != nil && (selected.audio == nil || selected.audio.Index != *options.AudioStream) {
		return clipStreams{}, fmt.Errorf("%w: audio stream %d not found", ErrInvalidStream, *options.AudioStream)
	}
	if options.SubtitleStream != nil {
		found := false
		for _, stream := range result.Streams {
			if stream.CodecType == "subtitle" && stream.Index == *options.SubtitleStream {
				found = true
				break
			}
		}
		if !found {
			return clipStreams{}, fmt.Errorf("%w: subtitle stream %d not found", ErrInvalidStream, *options.SubtitleStream)
		}
	}
	return selected, nil
}

func clipArgs(path, outputPath string, options ClipOptions, streams clipStreams) []string {
	start := formatDuration(options.Start)
	duration := formatDuration(options.Duration)
	args := []string{"-nostdin", "-v", "error", "-ss", start, "-i", path, "-t", duration, "-map", fmt.Sprintf("0:%d", streams.video.Index)}
	if streams.audio != nil {
		args = append(args, "-map", fmt.Sprintf("0:%d", streams.audio.Index))
	}

	if options.SubtitleStream != nil || options.SubtitlePath != "" {
		subtitlePath := options.SubtitlePath
		if subtitlePath == "" {
			subtitlePath = path
		}
		filter := fmt.Sprintf("setpts=PTS+%s/TB,subtitles=filename='%s'", start, escapeFilterPath(subtitlePath))
		if options.SubtitleStream != nil {
			filter += fmt.Sprintf(":si=%d", streams.subtitleOrdinal)
		}
		filter += fmt.Sprintf(",setpts=PTS-%s/TB", start)
		args = append(args, "-vf", filter, "-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p")
	} else {
		args = append(args, "-c:v", "copy", "-sn")
	}

	if streams.audio != nil {
		burningSubtitles := options.SubtitleStream != nil || options.SubtitlePath != ""
		if streams.audio.CodecName == "aac" && !burningSubtitles {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "aac", "-b:a", "192k")
		}
	}
	return append(args,
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-avoid_negative_ts", "make_zero",
		"-movflags", "+faststart",
		"-y", outputPath,
	)
}

func formatDuration(value time.Duration) string {
	return strconv.FormatFloat(value.Seconds(), 'f', 3, 64)
}

func escapeFilterPath(path string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		// Inside a quoted filter value, close the quote, escape the apostrophe,
		// and reopen the quote. A backslash alone does not escape a quote there.
		`'`, `'\''`,
		`:`, `\:`,
		`,`, `\,`,
		`;`, `\;`,
		`[`, `\[`,
		`]`, `\]`,
	)
	return replacer.Replace(path)
}

func isNoSpace(err error, message string) bool {
	lower := strings.ToLower(message)
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) ||
		strings.Contains(lower, "no space left on device") || strings.Contains(lower, "disk quota exceeded")
}

type boundedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) >= b.limit {
		b.buf.Reset()
		_, _ = b.buf.Write(p[len(p)-b.limit:])
		return len(p), nil
	}
	if overflow := b.buf.Len() + len(p) - b.limit; overflow > 0 {
		b.buf.Next(overflow)
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	return b.buf.String()
}

var _ io.Writer = (*boundedBuffer)(nil)
