package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"jellybrarian/ffmpeg"
)

const ffprobeTimeout = 30 * time.Second

func serveFFProbe(w http.ResponseWriter, r *http.Request, root string) {
	path, _, err := resolveScopedFile(r.URL.Query().Get("path"), root)
	if err != nil {
		if errors.Is(err, errFileNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), ffprobeTimeout)
	defer cancel()
	output, err := ffmpeg.Probe(ctx, path)
	if err != nil {
		log.Printf("ffprobe failed for %q: %v", path, err)
		if ctx.Err() != nil {
			http.Error(w, "ffprobe timed out", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}
