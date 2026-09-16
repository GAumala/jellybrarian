package server

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
)

func ValidateFFmpeg() error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg executable not found: %w", err)
	}
	return nil
}

func serveAudio(w http.ResponseWriter, r *http.Request, root string) {
	query := r.URL.Query()
	path, _, err := resolveScopedFile(query.Get("path"), root)
	if err != nil {
		if errors.Is(err, errFileNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	stream := query.Get("stream")
	if stream == "" {
		http.Error(w, "stream query parameter is required", http.StatusBadRequest)
		return
	}
	typeName := query.Get("type")
	if typeName != "raw" && typeName != "wav" {
		http.Error(w, "type must be raw or wav", http.StatusBadRequest)
		return
	}
	ext := query.Get("ext")
	if typeName == "raw" {
		if ext == "" {
			http.Error(w, "ext query parameter is required for raw output", http.StatusBadRequest)
			return
		}
		if ext != "aac" && ext != "m4a" {
			http.Error(w, "ext must be aac or m4a", http.StatusBadRequest)
			return
		}
	} else {
		ext = "wav"
	}

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		http.Error(w, "ffmpeg executable not found", http.StatusInternalServerError)
		return
	}
	args := []string{"-nostdin", "-v", "error", "-i", path, "-map", stream}
	if typeName == "raw" {
		args = append(args, "-c:a", "copy")
		if ext == "aac" {
			args = append(args, "-f", "adts")
		} else {
			// MP4 needs fragmented output because stdout is not seekable.
			args = append(args, "-f", "mp4", "-movflags", "frag_keyframe+empty_moov")
		}
	} else {
		args = append(args, "-ac", "1", "-ar", "22050", "-f", "wav")
	}
	args = append(args, "pipe:1")

	cmd := exec.CommandContext(r.Context(), ffmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr

	w.Header().Set("Content-Type", audioContentType(typeName, ext))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", generatedAudioName(path, ext)))
	if err := cmd.Run(); err != nil {
		// The response may already contain partial audio, so its status cannot be changed here.
		_ = stderr
	}
}

func audioContentType(typeName, ext string) string {
	if typeName == "wav" {
		return "audio/wav"
	}
	switch strings.ToLower(ext) {
	case "aac":
		return "audio/aac"
	case "m4a":
		return "audio/mp4"
	default:
		return "application/octet-stream"
	}
}

func generatedAudioName(path, ext string) string {
	base := filepath.Base(path)
	if originalExt := filepath.Ext(base); originalExt != "" {
		base = strings.TrimSuffix(base, originalExt)
	}
	return base + "." + ext
}
