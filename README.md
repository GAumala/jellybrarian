# media-manager

HTTP server for managing media files and organizing them into Jellyfin library directories via hard links.

## Requirements

- Go 1.22+
- All media and Jellyfin directories must be on the **same filesystem** (hard link requirement)

## Build

```bash
go build -o media-manager .
```

That's it. Produces a single self-contained binary with no runtime dependencies.

## Test

```bash
go test ./...
```

Verbose:

```bash
go test ./... -v
```

### Cross-compile for Linux (e.g. Raspberry Pi, home server)

```bash
# 64-bit ARM (Raspberry Pi 4)
GOOS=linux GOARCH=arm64 go build -o media-manager .

# 64-bit x86
GOOS=linux GOARCH=amd64 go build -o media-manager .
```

## Configuration

Copy and edit `config.toml`:

```toml
[directories]
media           = "/mnt/hdd0/media"       # where downloads and rips land
jellyfin_music  = "/mnt/hdd0/jellyfin/music"
jellyfin_movies = "/mnt/hdd0/jellyfin/movies"
jellyfin_tv     = "/mnt/hdd0/jellyfin/tv"
```

## Run

```bash
./media-manager
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `config.toml` | Path to config file |
| `-addr` | `:8090` | Listen address |

```bash
./media-manager -config /etc/media-manager/config.toml -addr :8090
```

## API

### `GET /media/list`

Lists all entries in the media directory, sorted oldest → newest (newest at the bottom).
Useful for spotting recently added files that still need to be organized into Jellyfin.

```bash
curl http://localhost:8090/media/list
```

```json
["Some Old Movie", "Another Thing", "Artist - New Album"]
```

---

### `PUT /media/artists/{artist}/organize`

Scans the media directory for folders matching `<artist> - <album>` and hard-links
their contents into the Jellyfin music library.

```bash
curl -X PUT http://localhost:8090/media/artists/Metallica/organize
```

**Naming convention in media dir:**
```
Artist Name - Album Title/
```

**Single-disc album** (directory contains files):
```
media/Metallica - Master of Puppets/  →  jellyfin/music/Metallica/Master of Puppets/
```

**Multi-disc album** (directory contains subdirectories):
The disc number is parsed from the subdirectory name (any integer found in the name).
```
media/Metallica - S&M/
  Disc 1/   →  jellyfin/music/Metallica/S&M/Disc 1/
  Disc 2/   →  jellyfin/music/Metallica/S&M/Disc 2/
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

Files that already exist at the destination are skipped (noted in the response).

## Project Structure

```
media-manager/
├── main.go          # entry point, CLI flags, starts server
├── config/
│   └── config.go    # TOML loading and validation
├── media/
│   └── media.go     # media listing and hard-link organization logic
├── server/
│   └── server.go    # HTTP route definitions
└── config.toml      # configuration file
```
