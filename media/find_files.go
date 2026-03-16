package media

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FindVideoFiles scans path for .mp4 and .mkv files. If path is a single file with
// .mp4 or .mkv extension, returns a slice containing only that path. If path is a
// directory, recursively walks it and returns all video file paths.
func FindVideoFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", path, err)
	}
	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if ext == ".mp4" || ext == ".mkv" {
			return []string{path}, nil
		}
		return nil, nil
	}

	var videos []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext == ".mp4" || ext == ".mkv" {
			videos = append(videos, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning %q: %w", path, err)
	}
	return videos, nil
}

// langTrackRegex matches .XX or .XXX plus .aac, .ac3, or .srt at end of filename (2 or 3-letter language code).
var langTrackRegex = regexp.MustCompile(`\.([a-zA-Z]{2,3})\.(aac|ac3|srt)$`)

// lang3to2 maps 3-letter ISO 639-2 codes to 2-letter for supported languages.
var lang3to2 = map[string]string{
	"spa": "es", "eng": "en", "ita": "it", "fra": "fr", "jpn": "ja",
	"por": "pt", "rus": "ru", "zho": "zh", "kor": "ko", 
}

// extraTrack is a language-tagged audio or subtitle file for a movie. lang is empty for unnamed .srt.
type extraTrack struct {
	path string
	lang string
	ext  string // e.g. "aac", "ac3", "srt"
}

// findMovieExtraTracks scans dir for audio (.XX.aac, .XX.ac3) and subtitle (.XX.srt or plain .srt) files.
// Accepts 2-letter codes and 3-letter codes (spa, eng, ita, fra, jpn) mapped to 2-letter.
// Files ending in .srt with no language code are included as unnamed subtitle.
func findMovieExtraTracks(dir string) ([]extraTrack, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var tracks []extraTrack
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		m := langTrackRegex.FindStringSubmatch(name)
		if m != nil {
			code := strings.ToLower(m[1])
			var lang string
			if len(code) == 2 {
				lang = code
			} else if two, ok := lang3to2[code]; ok {
				lang = two
			} else {
				continue // unsupported 3-letter code
			}
			tracks = append(tracks, extraTrack{
				path: filepath.Join(dir, name),
				lang: lang,
				ext:  m[2],
			})
			continue
		}
		if strings.HasSuffix(lower, ".srt") {
			tracks = append(tracks, extraTrack{
				path: filepath.Join(dir, name),
				lang: "",
				ext:  "srt",
			})
		}
	}
	return tracks, nil
}
