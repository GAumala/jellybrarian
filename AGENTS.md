# Adding Endpoints

## Project Structure

```
jellybrarian/
├── main.go              # entry point, CLI flags, starts server
├── config/
│   └── config.go        # TOML loading and validation
├── media/
│   ├── media.go         # MediaManager: listing and hard-link organization
│   ├── media_test.go    # tests for media.go
│   ├── find_files.go    # video file discovery, audio/subtitle track handling
│   ├── parser.go        # season/episode parsing for TV shows
│   └── parser_test.go   # tests for parser.go
├── server/
│   ├── server.go        # HTTP route definitions
│   ├── library.go       # LibraryKind, createMediaManager, lib-index resolution
│   └── input.go         # query parameter helpers
├── text/
│   └── text.go          # accent-insensitive string normalization for search
├── config.toml          # configuration file
└── AGENTS.md            # this file
```

## Steps to Add an Endpoint

### 1. Add the handler function in `server/server.go`

Routes are registered with `mux.HandleFunc`. Use the method pattern syntax `"METHOD /path"`.

```go
mux.HandleFunc("GET /media/movies/files", func(w http.ResponseWriter, r *http.Request) {
    mgr, err := createMediaManager(r, cfg, LibraryMovies)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    // ...
})
```

### 2. Use `createMediaManager` for library-scoped endpoints

`createMediaManager(r, cfg, LibraryMovies)` handles `lib-index` resolution. It returns a `media.MediaManager` configured with:
- `MediaDir`: from config (`media` key)
- `LibraryDir`: the resolved library path (`jellyfin_movies`, `jellyfin_tv`, or `jellyfin_music`)

For endpoints that only need the media dir (not a library), use `mediaManager(cfg, "")` instead.

### 3. Validate inputs early

Check required query params first, return 400 if missing:
```go
title := r.URL.Query().Get("title")
if title == "" {
    http.Error(w, "title query parameter is required", http.StatusBadRequest)
    return
}
```

For library title validation errors (`ErrInvalidLibraryTitle`), return 400:
```go
if errors.Is(err, media.ErrInvalidLibraryTitle) {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
}
```

### 4. Call MediaManager methods

All media operations live in `media/media.go` on `MediaManager`. Add new methods there as needed.

## Steps to Add Tests

### 1. Media logic belongs in `media/media.go`

Tests for `MediaManager` methods go in `media/media_test.go`. Tests for parsing utilities go in `media/parser_test.go`.

### 2. Use `testEnv` for temp directory structure

```go
func newTestEnv(t *testing.T) testEnv {
    root := t.TempDir()
    e := testEnv{
        Media:  filepath.Join(root, "media"),
        Movies: filepath.Join(root, "jellyfin", "movies"),
        TV:     filepath.Join(root, "jellyfin", "tv"),
    }
    for _, d := range []string{e.Media, e.Movies, e.TV} {
        if err := os.MkdirAll(d, 0755); err != nil {
            t.Fatalf("failed to create dir %s: %v", d, err)
        }
    }
    return e
}
```

Use the helper methods to get a `MediaManager`:
```go
mgr := env.mgrMovies()  // MediaManager{MediaDir: env.Media, LibraryDir: env.Movies}
mgr := env.mgrTV()       // MediaManager{MediaDir: env.Media, LibraryDir: env.TV}
```

Create test files with:
```go
createFile(t, filepath.Join(env.Movies, "Inception (2010)"), "Inception (2010).mkv", "video")
```

### 3. Test naming convention

- `TestListTitleFiles_Movies` - tests the movies variant
- `TestListTitleFiles_TV` - tests the TV variant
- `TestListTitleFiles_InvalidTitle` - tests error handling for invalid input
- `TestListTitleFiles_TitleNotFound` - tests error handling for missing title

### 4. Common error types to test

- `ErrInvalidLibraryTitle` - title is not a direct child of the library dir (e.g. `../escape`, empty string, or the library root itself)
- `TitleNotFound` - movie folder doesn't exist under the library dir
- `ErrNoVideoFiles` - no .mp4/.mkv files found at source path

## Running Tests

```bash
go test ./...           # all tests
go test ./media/        # media package only
go test ./media/ -v     # verbose output
go test ./media/ -v -run TestListTitleFiles  # specific test
```

## Running the Server

```bash
go build -o jellybrarian .
./jellybrarian -config config.toml -addr :8090
```

## Configuration

The `Config` struct holds:
- `Media` (string) - staging area for downloads
- `JellyfinMusic`, `JellyfinMovies`, `JellyfinTV` (each `[]string`) - library roots

Multiple library paths are supported via TOML arrays. The `lib-index` query param (default `0`) selects which path to use.

## Key Utilities

- `media.MediaManager.ListLibraryTitles(q string)` - list subdirectories, optionally filtered by search query
- `media.MediaManager.resolveLibraryTitleDir(title string)` - resolve `LibraryDir/title`, validates it's a direct child
- `media.FindVideoFiles(path string)` - recursively find .mp4/.mkv files
- `text.NormalizeForSearch(s string)` - accent-insensitive lowercase for search matching
- `server.queryInt(r *http.Request, key string, defaultVal int)` - parse integer query params