package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"jellybrarian/ffmpeg"
)

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

	w.Header().Set("Content-Type", audioContentType(typeName, ext))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", generatedAudioName(path, ext)))
	if err := ffmpeg.ExtractAudio(r.Context(), path, ffmpeg.AudioOptions{Type: typeName, Stream: stream, Ext: ext}, w); err != nil {
		// The response may already contain partial audio, so its status cannot be changed here.
		log.Printf("audio extraction failed for %q: %v", path, err)
		return
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
