# jellybrarian

HTTP server for managing media files and organizing them into Jellyfin library directories via hard links.

## Requirements

- Go 1.22+
- All media and Jellyfin directories must be on the **same filesystem** (hard link requirement)

## Build

```bash
go build -o jellybrarian .
```

That's it. Produces a single self-contained binary with no runtime dependencies.

## Test

```bash
go test ./...
```

### Cross-compile for Linux (e.g. Raspberry Pi)

```bash
# 64-bit ARM (Raspberry Pi 4)
GOOS=linux GOARCH=arm64 go build -o jellybrarian .

# 64-bit x86
GOOS=linux GOARCH=amd64 go build -o jellybrarian .
```

## Configuration

Copy and edit `config.toml`. 

| Key | Required | Description |
|-----|----------|-------------|
| `media` | **yes** | Root folder for downloads and staging (where new rips land). Must exist as a directory. |
| `jellyfin_music` | no | Music library root(s). Omit if you only use movies/TV. |
| `jellyfin_movies` | no | Movie library root(s). |
| `jellyfin_tv` | no | TV library root(s). |

Each `jellyfin_*` value may be either a **single string** or a **TOML array of strings** if you have multiple library roots for that type:

```toml
media = "/mnt/hdd0/media"

jellyfin_music  = "/mnt/hdd0/jellyfin/music"
jellyfin_movies = "/mnt/ssd/jellyfin/movies"
jellyfin_tv     = ["/mnt/hdd0/jellyfin/tv", "/mnt/hdd0/jellyfin/anime"]
```

Validation on startup:

- Every configured path must exist and be a directory.
- Empty strings are not allowed inside a `jellyfin_*` list.
- An empty list (`jellyfin_tv = []`) or omitting a key means you are not using that library type from the API (endpoints that need that list will return an error).

### Choosing a library at runtime (`lib-index`)

Most HTTP endpoints take an optional query parameter **`lib-index`** (integer, default **`0`**). It selects **which** path to use when `jellyfin_music`, `jellyfin_movies`, or `jellyfin_tv` is an array (or always `0` when there is only one path). **`GET /media/list` does not use `lib-index`** (it only lists `media`).

| Endpoint | Uses `jellyfin_*` |
|----------|-------------------|
| `GET /media/list` | — (`lib-index` ignored) |
| `GET /media/tv/titles` | `jellyfin_tv` |
| `GET /media/movies/titles` | `jellyfin_movies` |
| `PUT /media/artists/{artist}/organize` | `jellyfin_music` |
| `PUT /media/tv/add` | `jellyfin_tv` |
| `PUT /media/movies/add` | `jellyfin_movies` |

## Run

```bash
./jellybrarian
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `config.toml` | Path to config file |
| `-addr` | `:8090` | Listen address |

```bash
./jellybrarian -config /etc/jellybrarian/config.toml -addr :8090
```

## API

### `GET /media/list`

Lists entries in the **media** directory (`media` in config), sorted oldest → newest (newest at the bottom).
Useful for spotting recently added files that still need to be organized into Jellyfin.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `limit`   | no       | If `> 0`, only the **most recent** `limit` entries are returned (after sorting). Default `0` means no limit. |
| `lib-index` | —      | Ignored for this route. |

```bash
curl http://localhost:8090/media/list
curl "http://localhost:8090/media/list?limit=20"
```

```json
["Some Old Movie", "Another Thing", "Artist - New Album"]
```

---

### `GET /media/tv/titles`

Lists TV show titles (immediate subdirectory names under the selected **TV** library path), sorted alphabetically.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `q`       | no       | Filter by keywords: case- and accent-insensitive. All space-separated terms must appear in the title (e.g. `one piece` matches "ONE PIECE (2023)"). |
| `lib-index` | no   | Which `jellyfin_tv` path to use (default `0`). |

```bash
curl http://localhost:8090/media/tv/titles
curl "http://localhost:8090/media/tv/titles?q=one%20piece"
curl "http://localhost:8090/media/tv/titles?lib-index=1"
```

```json
["Breaking Bad (2008)", "ONE PIECE (2023)", "Succession"]
```

---

### `GET /media/movies/titles`

Lists movie titles (immediate subdirectory names under the selected **movies** library path), sorted alphabetically.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `q`       | no       | Filter by keywords: case- and accent-insensitive. All space-separated terms must appear in the title. |
| `lib-index` | no   | Which `jellyfin_movies` path to use (default `0`). |

```bash
curl http://localhost:8090/media/movies/titles
curl "http://localhost:8090/media/movies/titles?q=inception"
```

```json
["Inception (2010)", "The Lord of the Rings (2001)"]
```

---

### `PUT /media/artists/{artist}/organize`

Scans the media directory for folders matching `<artist> - <album>` and hard-links
their contents into the selected **music** Jellyfin library path.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `lib-index` | no   | Which `jellyfin_music` path to use (default `0`). |

```bash
curl -X PUT http://localhost:8090/media/artists/Metallica/organize
curl -X PUT "http://localhost:8090/media/artists/Metallica/organize?lib-index=1"
```

**Single-disc album** (directory contains files):
```
media/Metallica - Master of Puppets/  →  jellyfin/music/Metallica/Master of Puppets/
```

**Multi-disc album** (directory contains subdirectories):
The disc number is parsed from the subdirectory name (any integer found in the name).
```
media/Metallica - S&M/
  CD 1/   →  jellyfin/music/Metallica/S&M/Disc 1/
  CD 2/   →  jellyfin/music/Metallica/S&M/Disc 2/
```

Response:
```json
{
  "artist": "Metallica",
  "linked": [
    "/mnt/hdd0/jellyfin/music/Metallica/Master of Puppets/01 - Battery.flac",
    "/mnt/hdd0/jellyfin/music/Metallica/Master of Puppets/02 - Master of Puppets.flac"
  ]
}
```

If a destination file already exists, it is removed and replaced by the new hard link.

---

### `PUT /media/tv/add`

Finds video files at the given path (file or directory under the media root), parses
season/episode from filenames, and hard-links them into the selected **TV** Jellyfin library as
`{title}/Season N/{title} - S01E01.ext`.

**Query parameters:**

| Parameter   | Required | Description                                      |
|------------|----------|--------------------------------------------------|
| `media-path` | yes     | Path under media dir (e.g. `Breaking Bad` or `Show/Season 1`) |
| `title`      | yes     | Jellyfin show title (e.g. `Breaking Bad (2008)`) |
| `lib-index` | no   | Which `jellyfin_tv` path to use (default `0`). |

```bash
curl -X PUT "http://localhost:8090/media/tv/add?media-path=Breaking%20Bad&title=Breaking%20Bad%20(2008)"
curl -X PUT "http://localhost:8090/media/tv/add?media-path=Breaking%20Bad&title=Breaking%20Bad%20(2008)&lib-index=1"
```

Response:
```json
{
  "media_path": "Breaking Bad",
  "title": "Breaking Bad (2008)",
  "linked": ["/mnt/hdd0/jellyfin/tv/Breaking Bad (2008)/Season 1/Breaking Bad (2008) - S01E01.mkv", "..."]
}
```

Files that cannot be parsed for season/episode are skipped. Existing destination files are replaced when linking.

---

### `PUT /media/movies/add`

Finds video file(s) at the given path (single file or directory under the media root)
and hard-links them into the selected **movies** Jellyfin library.

- **Single file:** `{title}/{title}.ext`
- **Multiple files:** `{title}/{title}-part-1.ext`, `{title}-part-2.ext`, ...

**Query parameters:**

| Parameter   | Required | Description                                      |
|------------|----------|--------------------------------------------------|
| `media-path` | yes     | Path under media dir (file or directory, e.g. `Inception.2010.mkv` or `Lord of the Rings`) |
| `title`      | yes     | Jellyfin movie title (e.g. `Inception (2010)`)   |
| `lib-index` | no   | Which `jellyfin_movies` path to use (default `0`). |

```bash
curl -X PUT "http://localhost:8090/media/movies/add?media-path=Inception.2010.mkv&title=Inception%20(2010)"
```

Response:
```json
{
  "media_path": "Inception.2010.mkv",
  "title": "Inception (2010)",
  "linked": ["/mnt/hdd0/jellyfin/movies/Inception (2010)/Inception (2010).mkv"]
}
```

Existing destination files are replaced when linking.

---

## Deploy with Docker

Edit `config.toml` with your actual paths, then on the Pi:

```bash
git clone <repo> && cd jellybrarian
docker build -t jellybrarian .
docker run -d \
  --name jellybrarian \
  --restart unless-stopped \
  --user 1000 \
  -p 8296:8090 \
  -v "$(pwd)/config.toml:/app/config.toml:ro" \
  -v /mnt/hdd0:/mnt/hdd0 \
  jellybrarian
```

All paths in `config.toml` should be absolute paths as they appear on the host
(e.g. `/mnt/hdd0/media`). The service is accessible at `http://raspberry.local:8296`.

> **Note:** Hard links require source and destination to be on the same filesystem.
> As long as everything under `/mnt/hdd0` is one volume, this works fine inside
> the container since the whole mount is shared.

## Project Structure

```
jellybrarian/
├── main.go              # entry point, CLI flags, starts server
├── config/
│   └── config.go        # TOML loading and validation
├── media/
│   ├── media.go         # MediaManager: listing and hard-link organization
│   └── media_test.go    # tests
├── server/
│   ├── server.go        # HTTP route definitions
│   ├── library.go       # LibraryKind, createMediaManager, lib-index resolution
│   └── input.go         # query parameter helpers
├── Dockerfile
└── config.toml
```
