package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	errFilePathRequired = errors.New("path query parameter is required")
	errFilePathRelative = errors.New("path must be absolute")
	errFileNotFound     = errors.New("file not found")
	errFileIsNotRegular = errors.New("path must refer to a file")
	errFileOutsideRoot  = errors.New("path is outside the configured library")
	errFileExists       = errors.New("file already exists")
)

func resolveScopedFile(path, root string) (string, os.FileInfo, error) {
	if path == "" {
		return "", nil, errFilePathRequired
	}
	if !filepath.IsAbs(path) {
		return "", nil, errFilePathRelative
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, errFileNotFound
		}
		return "", nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, errFileIsNotRegular
	}

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve file root: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve file path: %w", err)
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", nil, errFileOutsideRoot
	}
	return resolvedPath, info, nil
}

func serveScopedFile(w http.ResponseWriter, r *http.Request, root string) {
	path := r.URL.Query().Get("path")
	path, info, err := resolveScopedFile(path, root)
	if err != nil {
		if errors.Is(err, errFileNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if errors.Is(err, errFilePathRequired) || errors.Is(err, errFilePathRelative) || errors.Is(err, errFileIsNotRegular) || errors.Is(err, errFileOutsideRoot) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	file, err := os.Open(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to open file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}

func resolveScopedUploadPath(path, root string) (string, error) {
	if path == "" {
		return "", errFilePathRequired
	}
	if !filepath.IsAbs(path) {
		return "", errFilePathRelative
	}

	path = filepath.Clean(path)
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file root: %w", err)
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", fmt.Errorf("failed to resolve target directory: %w", err)
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedParent)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errFileOutsideRoot
	}

	return filepath.Join(resolvedParent, filepath.Base(path)), nil
}

func uploadScopedFile(w http.ResponseWriter, r *http.Request, root string) {
	path, err := resolveScopedUploadPath(r.URL.Query().Get("path"), root)
	if err != nil {
		if errors.Is(err, errFilePathRequired) || errors.Is(err, errFilePathRelative) || errors.Is(err, errFileOutsideRoot) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			http.Error(w, errFileExists.Error(), http.StatusConflict)
		} else {
			http.Error(w, fmt.Sprintf("failed to create file: %v", err), http.StatusInternalServerError)
		}
		return
	}

	if _, err := io.Copy(file, r.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		http.Error(w, fmt.Sprintf("failed to write file: %v", err), http.StatusInternalServerError)
		return
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		http.Error(w, fmt.Sprintf("failed to close file: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func deleteScopedFile(w http.ResponseWriter, r *http.Request, root string) {
	path := r.URL.Query().Get("path")
	path, _, err := resolveScopedFile(path, root)
	if err != nil {
		if errors.Is(err, errFileNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if errors.Is(err, errFilePathRequired) || errors.Is(err, errFilePathRelative) || errors.Is(err, errFileIsNotRegular) || errors.Is(err, errFileOutsideRoot) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if err := os.Remove(path); err != nil {
		http.Error(w, fmt.Sprintf("failed to delete file: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
