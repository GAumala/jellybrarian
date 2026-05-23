package server

import (
	"encoding/json"
	"errors"
	"io"
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
		mgr := mediaManager(cfg, "")

		names, err := mgr.ListMedia(limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(names)
	})

	mux.HandleFunc("GET /media/files", func(w http.ResponseWriter, r *http.Request) {
		title := r.URL.Query().Get("title")
		if title == "" {
			http.Error(w, "title query parameter is required", http.StatusBadRequest)
			return
		}
		mgr := mediaManager(cfg, "")

		files, err := mgr.ListMediaFiles(title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})

	mux.HandleFunc("GET /media/tv/titles", func(w http.ResponseWriter, r *http.Request) {
		mgr, err := createMediaManager(r, cfg, LibraryTV)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		q := r.URL.Query().Get("q")
		names, err := mgr.ListLibraryTitles(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(names)
	})

	mux.HandleFunc("GET /media/movies/titles", func(w http.ResponseWriter, r *http.Request) {
		mgr, err := createMediaManager(r, cfg, LibraryMovies)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		q := r.URL.Query().Get("q")
		names, err := mgr.ListLibraryTitles(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(names)
	})

	mux.HandleFunc("GET /media/movies/files", func(w http.ResponseWriter, r *http.Request) {
		mgr, err := createMediaManager(r, cfg, LibraryMovies)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		title := r.URL.Query().Get("title")
		if title == "" {
			http.Error(w, "title query parameter is required", http.StatusBadRequest)
			return
		}

		files, err := mgr.ListTitleFiles(title)
		if err != nil {
			if errors.Is(err, media.ErrInvalidLibraryTitle) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})

	mux.HandleFunc("GET /media/tv/files", func(w http.ResponseWriter, r *http.Request) {
		mgr, err := createMediaManager(r, cfg, LibraryTV)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		title := r.URL.Query().Get("title")
		if title == "" {
			http.Error(w, "title query parameter is required", http.StatusBadRequest)
			return
		}

		files, err := mgr.ListTitleFiles(title)
		if err != nil {
			if errors.Is(err, media.ErrInvalidLibraryTitle) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})

	mux.HandleFunc("PUT /media/artists/{artist}/organize", func(w http.ResponseWriter, r *http.Request) {
		mgr, err := createMediaManager(r, cfg, LibraryMusic)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		artist := r.PathValue("artist")
		if artist == "" {
			http.Error(w, "artist is required", http.StatusBadRequest)
			return
		}

		linked, err := mgr.OrganizeArtist(artist)
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
		mgr, err := createMediaManager(r, cfg, LibraryTV)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

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

		files := parseFilesBody(r)

		linked, err := mgr.AddTVSeason(mediaPath, title, files)
		if err != nil {
			var nv *media.ErrNoVideoFiles
			if errors.As(err, &nv) {
				writeErrNoVideoFiles(w, nv)
				return
			}
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
		mgr, err := createMediaManager(r, cfg, LibraryMovies)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

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

		files := parseFilesBody(r)

		linked, err := mgr.AddMovie(mediaPath, title, files)
		if err != nil {
			var nv *media.ErrNoVideoFiles
			if errors.As(err, &nv) {
				writeErrNoVideoFiles(w, nv)
				return
			}
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

	mux.HandleFunc("PUT /media/movies/subtitles", func(w http.ResponseWriter, r *http.Request) {
		mgr, err := createMediaManager(r, cfg, LibraryMovies)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		title := r.URL.Query().Get("title")
		lang := r.URL.Query().Get("lang")

		if title == "" {
			http.Error(w, "title query parameter is required", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		path, err := mgr.SubtitleMovie(title, lang, string(body))
		if err != nil {
			if errors.Is(err, media.TitleNotFound) || errors.Is(err, media.ErrInvalidLibraryTitle) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"title": title,
			"lang": lang,
			"path": path,
		})
	})

	mux.HandleFunc("PUT /media/tv/delist", func(w http.ResponseWriter, r *http.Request) {
		mgr, err := createMediaManager(r, cfg, LibraryTV)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		title := r.URL.Query().Get("title")
		if title == "" {
			http.Error(w, "title query parameter is required", http.StatusBadRequest)
			return
		}

		if err := mgr.DelistTitle(title); err != nil {
			if errors.Is(err, media.ErrInvalidLibraryTitle) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"title": title,
		})
	})

	mux.HandleFunc("PUT /media/movies/delist", func(w http.ResponseWriter, r *http.Request) {
		mgr, err := createMediaManager(r, cfg, LibraryMovies)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		title := r.URL.Query().Get("title")
		if title == "" {
			http.Error(w, "title query parameter is required", http.StatusBadRequest)
			return
		}

		if err := mgr.DelistTitle(title); err != nil {
			if errors.Is(err, media.ErrInvalidLibraryTitle) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"title": title,
		})
	})

	return mux
}

func writeErrNoVideoFiles(w http.ResponseWriter, nv *media.ErrNoVideoFiles) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   nv.Error(),
		"src_dir": nv.SrcDir,
		"files":   nv.Files,
	})
}
