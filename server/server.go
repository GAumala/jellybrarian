package server

import (
	"encoding/json"
	"net/http"

	"github.com/gabriel/media-manager/config"
	"github.com/gabriel/media-manager/media"
)

func New(cfg *config.Config) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /media/list", func(w http.ResponseWriter, r *http.Request) {
		names, err := media.List(cfg.Directories)
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

	return mux
}
