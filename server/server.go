package server

import (
	"encoding/json"
	"net/http"

	"jellybrarian/config"
	"jellybrarian/media"
)

func New(cfg *config.Config) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /media/list", func(w http.ResponseWriter, r *http.Request) {
		limit, err := queryInt(r, "limit", 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		names, err := media.List(cfg.Directories, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(names)
	})

	mux.HandleFunc("PUT /media/artists/{artist}/organize", func(w http.ResponseWriter, r *http.Request) {
		artist := r.PathValue("artist")
		if artist == "" {
			http.Error(w, "artist is required", http.StatusBadRequest)
			return
		}

		linked, err := media.OrganizeArtist(cfg.Directories, artist)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"artist": artist,
			"linked": linked,
		})
	})

	mux.HandleFunc("PUT /media/tv/add", func(w http.ResponseWriter, r *http.Request) {
		mediaPath := r.URL.Query().Get("media-path")
		title := r.URL.Query().Get("title")

		if mediaPath == "" {
			http.Error(w, "media-path query parameter is required", http.StatusBadRequest)
			return
		}
		if title == "" {
			http.Error(w, "title query parameter is required", http.StatusBadRequest)
			return
		}

		linked, err := media.AddTVSeason(cfg.Directories, mediaPath, title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"media_path": mediaPath,
			"title":      title,
			"linked":     linked,
		})
	})

	mux.HandleFunc("PUT /media/movies/add", func(w http.ResponseWriter, r *http.Request) {
		mediaPath := r.URL.Query().Get("media-path")
		title := r.URL.Query().Get("title")

		if mediaPath == "" {
			http.Error(w, "media-path query parameter is required", http.StatusBadRequest)
			return
		}
		if title == "" {
			http.Error(w, "title query parameter is required", http.StatusBadRequest)
			return
		}

		linked, err := media.AddMovie(cfg.Directories, mediaPath, title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"media_path": mediaPath,
			"title":      title,
			"linked":     linked,
		})
	})

	return mux
}
