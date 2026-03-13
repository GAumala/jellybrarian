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

// langTrackRegex matches .XX.aac, .XX.ac3, or .XX.srt at end of filename (XX = 2-letter language code).
var langTrackRegex = regexp.MustCompile(`\.([a-zA-Z]{2})\.(aac|ac3|srt)$`)

// extraTrack is a language-tagged audio or subtitle file for a movie.
type extraTrack struct {
	path string
	lang string
	ext  string // e.g. "aac", "ac3", "srt"
}

// findMovieExtraTracks scans dir for audio (.XX.aac, .XX.ac3) and subtitle (.XX.srt) files
// with a 2-letter language code and returns them for linking.
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
		m := langTrackRegex.FindStringSubmatch(name)
		if m != nil {
			tracks = append(tracks, extraTrack{
				path: filepath.Join(dir, name),
				lang: strings.ToLower(m[1]),
				ext:  m[2],
			})
		}
	}
	return tracks, nil
}
