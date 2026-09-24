package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
)

// AudioOptions describes the audio stream and output format for extraction.
type AudioOptions struct {
	Type   string
	Stream string
	Ext    string
}

func Probe(ctx context.Context, path string) ([]byte, error) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("%w: ffprobe: %v", ErrExecutableMissing, err)
	}
	cmd := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_format", "-show_streams", "-of", "json", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}
	return output, nil
}

func ExtractAudio(ctx context.Context, path string, options AudioOptions, dst io.Writer) error {
	args, err := audioArgs(path, options)
	if err != nil {
		return err
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg executable not found: %w", err)
	}
	log.Printf("executing ffmpeg: %s", formatCommand(ffmpeg, args))
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stdout = dst
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return fmt.Errorf("ffmpeg failed: %w: %s", err, message)
		}
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}

func formatCommand(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(name))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func audioArgs(path string, options AudioOptions) ([]string, error) {
	if options.Stream == "" {
		return nil, fmt.Errorf("stream is required")
	}
	if options.Type != "raw" && options.Type != "wav" {
		return nil, fmt.Errorf("type must be raw or wav")
	}
	if options.Type == "raw" && options.Ext != "aac" && options.Ext != "ac3" && options.Ext != "m4a" {
		return nil, fmt.Errorf("ext must be aac, ac3, or m4a for raw output")
	}

	args := []string{"-nostdin", "-v", "error", "-i", path, "-map", options.Stream}
	switch options.Type {
	case "wav":
		return append(args, "-ac", "1", "-ar", "22050", "-f", "wav", "pipe:1"), nil
	case "raw":
		args = append(args, "-c:a", "copy")
		switch options.Ext {
		case "aac":
			return append(args, "-f", "adts", "pipe:1"), nil
		case "ac3":
			return append(args, "-f", "ac3", "pipe:1"), nil
		case "m4a":
			// Fragmented MP4 can be written to stdout without seeking.
			return append(args, "-f", "mp4", "-movflags", "frag_keyframe+empty_moov+default_base_moof", "-avoid_negative_ts", "make_zero", "pipe:1"), nil
		}
	}
	return nil, fmt.Errorf("unsupported audio output: type=%q ext=%q", options.Type, options.Ext)
}
