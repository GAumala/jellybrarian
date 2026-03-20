package server

import (
	"fmt"
	"net/http"

	"jellybrarian/config"
	"jellybrarian/media"
)

// LibraryKind selects which jellyfin_* path list in config applies to lib-index.
type LibraryKind int

const (
	LibraryMusic LibraryKind = iota
	LibraryMovies
	LibraryTV
)

func libraryPaths(cfg *config.Config, kind LibraryKind) (paths []string, field string) {
	switch kind {
	case LibraryMusic:
		return cfg.JellyfinMusic, "jellyfin_music"
	case LibraryMovies:
		return cfg.JellyfinMovies, "jellyfin_movies"
	case LibraryTV:
		return cfg.JellyfinTV, "jellyfin_tv"
	default:
		return nil, ""
	}
}

// createMediaManager reads lib-index from the request (default 0), resolves the
// library directory for the given kind, and returns a MediaManager. GET /media/list
// should use mediaManager(cfg, "") instead.
func createMediaManager(r *http.Request, cfg *config.Config, kind LibraryKind) (media.MediaManager, error) {
	idx, err := queryInt(r, "lib-index", 0)
	if err != nil {
		return media.MediaManager{}, err
	}
	paths, field := libraryPaths(cfg, kind)
	if field == "" {
		return media.MediaManager{}, fmt.Errorf("invalid library kind")
	}
	libDir, err := requireLibPath(paths, field, idx)
	if err != nil {
		return media.MediaManager{}, err
	}
	return mediaManager(cfg, libDir), nil
}

func requireLibPath(paths []string, field string, idx int) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("no %s libraries configured", field)
	}
	if idx < 0 || idx >= len(paths) {
		return "", fmt.Errorf("%s: lib-index %d out of range (have %d libraries, use 0..%d)", field, idx, len(paths), len(paths)-1)
	}
	return paths[idx], nil
}

func mediaManager(cfg *config.Config, libDir string) media.MediaManager {
	return media.MediaManager{MediaDir: cfg.Media, LibraryDir: libDir}
}
