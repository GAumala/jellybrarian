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

Verbose:

```bash
go test ./... -v
```

### Cross-compile for Linux (e.g. Raspberry Pi, home server)

```bash
# 64-bit ARM (Raspberry Pi 4)
GOOS=linux GOARCH=arm64 go build -o jellybrarian .

# 64-bit x86
GOOS=linux GOARCH=amd64 go build -o jellybrarian .
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

---

### `PUT /media/tv/add`

Finds video files at the given path (file or directory under the media root), parses
season/episode from filenames, and hard-links them into the Jellyfin TV library as
`{title}/Season N/{title} - S01E01.ext`.

**Query parameters:**

| Parameter   | Required | Description                                      |
|------------|----------|--------------------------------------------------|
| `media-path` | yes     | Path under media dir (e.g. `Breaking Bad` or `Show/Season 1`) |
| `title`      | yes     | Jellyfin show title (e.g. `Breaking Bad (2008)`) |

```bash
curl -X PUT "http://localhost:8090/media/tv/add?media-path=Breaking%20Bad&title=Breaking%20Bad%20(2008)"
```

Response:
```json
{
  "media_path": "Breaking Bad",
  "title": "Breaking Bad (2008)",
  "linked": ["/mnt/hdd0/jellyfin/tv/Breaking Bad (2008)/Season 1/Breaking Bad (2008) - S01E01.mkv", "..."]
}
```

Files that cannot be parsed for season/episode are skipped. Existing destination files are skipped (noted in the response).

---

### `PUT /media/movies/add`

Finds video file(s) at the given path (single file or directory under the media root)
and hard-links them into the Jellyfin movies library.

- **Single file:** `{title}/{title}.ext`
- **Multiple files:** `{title}/{title}-part-1.ext`, `{title}-part-2.ext`, ...

**Query parameters:**

| Parameter   | Required | Description                                      |
|------------|----------|--------------------------------------------------|
| `media-path` | yes     | Path under media dir (file or directory, e.g. `Inception.2010.mkv` or `Lord of the Rings`) |
| `title`      | yes     | Jellyfin movie title (e.g. `Inception (2010)`)   |

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

Existing destination files are skipped (noted in the response).

---

## Deploy with Docker (Raspberry Pi)

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
│   └── media.go         # media listing and hard-link organization logic
│   └── media_test.go    # tests
├── server/
│   └── server.go        # HTTP route definitions
├── Dockerfile
└── config.toml
```
