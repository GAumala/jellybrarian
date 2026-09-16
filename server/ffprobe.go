package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

const ffprobeTimeout = 30 * time.Second

// ValidateFFProbe verifies that the runtime dependency is available.
func ValidateFFProbe() error {
	_, err := exec.LookPath("ffprobe")
	if err != nil {
		return fmt.Errorf("ffprobe executable not found: %w", err)
	}
	return nil
}

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

	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		http.Error(w, "ffprobe executable not found", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), ffprobeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_format", "-show_streams", "-of", "json", path)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			http.Error(w, "ffprobe timed out", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, fmt.Sprintf("ffprobe failed: %v", err), http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}
