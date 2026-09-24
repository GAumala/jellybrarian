package server

import (
	"errors"
	"fmt"
	"log"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jellybrarian/ffmpeg"
)

const maxClipDuration = 5 * time.Minute

func serveClip(w http.ResponseWriter, r *http.Request, root string) {
	if r.Method == http.MethodHead {
		http.Error(w, "HEAD is not supported for generated clips", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query()
	path, _, err := resolveScopedFile(query.Get("path"), root)
	if err != nil {
		writeScopedFileError(w, err)
		return
	}

	start, err := parseClipTime(query.Get("start"), "start")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	duration, err := parseClipTime(query.Get("duration"), "duration")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if start < 0 {
		http.Error(w, "start must not be negative", http.StatusBadRequest)
		return
	}
	if duration <= 0 {
		http.Error(w, "duration must be greater than zero", http.StatusBadRequest)
		return
	}
	if duration < time.Millisecond {
		http.Error(w, "duration must be at least 1 millisecond", http.StatusBadRequest)
		return
	}
	if duration > maxClipDuration {
		http.Error(w, "duration must not exceed 5 minutes", http.StatusBadRequest)
		return
	}

	audioStream, err := optionalStreamIndex(query.Get("audio-stream"), "audio-stream")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	subtitleStream, err := optionalStreamIndex(query.Get("subtitle-stream"), "subtitle-stream")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	subtitlePath := query.Get("subtitle-path")
	if subtitleStream != nil && subtitlePath != "" {
		http.Error(w, "subtitle-stream and subtitle-path are mutually exclusive", http.StatusBadRequest)
		return
	}
	if subtitlePath != "" {
		subtitlePath, _, err = resolveScopedFile(subtitlePath, root)
		if err != nil {
			writeScopedFileError(w, err)
			return
		}
	}

	outputPath, err := ffmpeg.CreateClip(r.Context(), path, ffmpeg.ClipOptions{
		Start:          start,
		Duration:       duration,
		AudioStream:    audioStream,
		SubtitleStream: subtitleStream,
		SubtitlePath:   subtitlePath,
	})
	if err != nil {
		log.Printf("clip creation failed for %q: %v", path, err)
		switch {
		case errors.Is(err, ffmpeg.ErrNoSpace):
			http.Error(w, "insufficient storage for clip", http.StatusInsufficientStorage)
		case errors.Is(err, ffmpeg.ErrInvalidStream):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, ffmpeg.ErrExecutableMissing):
			http.Error(w, err.Error(), http.StatusInternalServerError)
		case errors.Is(err, r.Context().Err()):
			return
		default:
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		}
		return
	}
	defer func() {
		if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
			log.Printf("failed to remove clip %q: %v", outputPath, err)
		}
	}()

	file, err := os.Open(outputPath)
	if err != nil {
		http.Error(w, "failed to open generated clip", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "failed to stat generated clip", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": generatedClipName(path)}))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, generatedClipName(path), info.ModTime(), file)
}

func writeScopedFileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errFileNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, errFilePathRequired), errors.Is(err, errFilePathRelative), errors.Is(err, errFileIsNotRegular), errors.Is(err, errFileOutsideRoot):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parseClipTime(value, name string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("%s query parameter is required", name)
	}
	var seconds float64
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		if len(parts) != 3 {
			return 0, fmt.Errorf("%s must be seconds or HH:MM:SS[.mmm]", name)
		}
		hours, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("%s must be seconds or HH:MM:SS[.mmm]", name)
		}
		minutes, err := strconv.ParseUint(parts[1], 10, 8)
		if err != nil || minutes >= 60 {
			return 0, fmt.Errorf("%s contains invalid minutes", name)
		}
		last, err := strconv.ParseFloat(parts[2], 64)
		if err != nil || last < 0 || last >= 60 {
			return 0, fmt.Errorf("%s contains invalid seconds", name)
		}
		seconds = float64(hours)*3600 + float64(minutes)*60 + last
	} else {
		var err error
		seconds, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("%s must be seconds or HH:MM:SS[.mmm]", name)
		}
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || math.Abs(seconds) > float64(math.MaxInt64)/float64(time.Second) {
		return 0, fmt.Errorf("%s is out of range", name)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func optionalStreamIndex(value, name string) (*int, error) {
	if value == "" {
		return nil, nil
	}
	index, err := strconv.Atoi(value)
	if err != nil || index < 0 {
		return nil, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return &index, nil
}

func generatedClipName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return base + "-clip.mp4"
}
