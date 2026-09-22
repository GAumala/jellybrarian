# jellybrarian

HTTP server for managing media files and organizing them into Jellyfin library directories via hard links.

## Requirements

- Go 1.22+
- All media and Jellyfin directories must be on the **same filesystem** (hard link requirement)

The `ffmpeg` and `ffprobe` executables are optional. The server starts without them, but the audio extraction and media inspection endpoints return an error until the corresponding tools are installed.

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
| `auth_token` | no | Secret token required on every HTTP request when set. Omit or leave empty to disable authentication. |
| `jellyfin_music` | no | Music library root(s). Omit if you only use movies/TV. |
| `jellyfin_movies` | no | Movie library root(s). |
| `jellyfin_tv` | no | TV library root(s). |

Each `jellyfin_*` value may be either a **single string** or a **TOML array of strings** if you have multiple library roots for that type:

```toml
media = "/mnt/hdd0/media"
auth_token = "replace-with-a-long-random-secret"

jellyfin_music  = "/mnt/hdd0/jellyfin/music"
jellyfin_movies = "/mnt/ssd/jellyfin/movies"
jellyfin_tv     = ["/mnt/hdd0/jellyfin/tv", "/mnt/hdd0/jellyfin/anime"]
```

Validation on startup:

- Every configured path must exist and be a directory.
- Empty strings are not allowed inside a `jellyfin_*` list.
- An empty list (`jellyfin_tv = []`) or omitting a key means you are not using that library type from the API (endpoints that need that list will return an error).

## Authentication

When `auth_token` is set, every endpoint requires the configured token in one of these headers:

```bash
curl -H "Authorization: Bearer $JELLYBRARIAN_TOKEN" http://localhost:8090/media/list
curl -H "X-Jellybrarian-Token: $JELLYBRARIAN_TOKEN" http://localhost:8090/media/list
```

### Choosing a library at runtime (`lib-index`)

Most HTTP endpoints take an optional query parameter **`lib-index`** (integer, default **`0`**). It selects **which** path to use when `jellyfin_music`, `jellyfin_movies`, or `jellyfin_tv` is an array (or always `0` when there is only one path). **`GET /media/list` does not use `lib-index`** (it only lists `media`).

| Endpoint | Uses `jellyfin_*` |
|----------|-------------------|
| `GET /media/list` | — (`lib-index` ignored) |
| `GET /media/files` | — (`lib-index` ignored) |
| `GET /media/file` | — (`lib-index` ignored) |
| `GET /media/ffprobe` | — (`lib-index` ignored) |
| `GET /media/audio` | — (`lib-index` ignored) |
| `GET /media/tv/titles` | `jellyfin_tv` |
| `GET /media/movies/titles` | `jellyfin_movies` |
| `GET /media/tv/files` | `jellyfin_tv` |
| `GET /media/movies/files` | `jellyfin_movies` |
| `GET /media/tv/file` | `jellyfin_tv` |
| `GET /media/movies/file` | `jellyfin_movies` |
| `GET /media/tv/ffprobe` | `jellyfin_tv` |
| `GET /media/movies/ffprobe` | `jellyfin_movies` |
| `GET /media/tv/audio` | `jellyfin_tv` |
| `GET /media/movies/audio` | `jellyfin_movies` |
| `PUT /media/artists/{artist}/organize` | `jellyfin_music` |
| `PUT /media/artists/{artist}/delist` | `jellyfin_music` |
| `PUT /media/tv/add` | `jellyfin_tv` |
| `PUT /media/tv/delist` | `jellyfin_tv` |
| `PUT /media/movies/add` | `jellyfin_movies` |
| `PUT /media/movies/subtitles` | `jellyfin_movies` |
| `PUT /media/movies/delist` | `jellyfin_movies` |

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

### `GET /media/files`

Lists all files recursively under a direct child of the **media** directory.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `title` | yes | Name of a direct child folder under `media`. |
| `lib-index` | — | Ignored for this route. |

```bash
curl "http://localhost:8090/media/files?title=Breaking%20Bad"
```

Response:
```json
["/mnt/hdd0/media/Breaking Bad/episode-1.mkv", "/mnt/hdd0/media/Breaking Bad/episode-2.mkv"]
```

---

### `GET /media/file`

Fetches one file from the **media** directory for inspection. The `path` value should be an absolute path returned by `GET /media/files` and must refer to a file under the configured `media` root.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | yes | Absolute file path under the configured `media` directory. |
| `lib-index` | — | Ignored for this route. |

The response body is the raw file contents. The endpoint supports normal HTTP content-type detection and range requests.

```bash
curl "http://localhost:8090/media/file?path=%2Fmnt%2Fhdd0%2Fmedia%2FBreaking%20Bad%2Fepisode-1.mkv" \
  -o episode-1.mkv
```

---

### `GET /media/ffprobe`

Returns `ffprobe` metadata for one file under the configured **media** directory as JSON. The `path` value should be an absolute path returned by `GET /media/files`.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | yes | Absolute file path under the configured `media` directory. |
| `lib-index` | — | Ignored for this route. |

```bash
curl "http://localhost:8090/media/ffprobe?path=%2Fmnt%2Fhdd0%2Fmedia%2FBreaking%20Bad%2Fepisode-1.mkv"
```

---

### `GET /media/audio`

Extracts one audio stream from a video file under the configured **media** directory and streams it in the response. The `path` value should be an absolute path returned by `GET /media/files`.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | yes | Absolute video file path under the configured `media` directory. |
| `type` | yes | `raw` to copy the stream without re-encoding, or `wav` to convert it to mono 22050 Hz WAV. |
| `stream` | yes | ffmpeg stream map, such as `0:1` or `0:a:1`. |
| `ext` | raw only | Output extension: `aac`, `ac3`, or `m4a`. |
| `lib-index` | — | Ignored for this route. |

The response is streamed directly from ffmpeg and is not written to the server filesystem. For `raw`, `aac` is written as ADTS and `m4a` as fragmented MP4, both with `-c:a copy`. For `wav`, ffmpeg uses `-ac 1 -ar 22050 -f wav`.

The exact ffmpeg commands used are listed in [Audio Extraction Commands](#audio-extraction-commands). The TV and movie audio endpoints build the same commands against their selected library roots.

```bash
curl "http://localhost:8090/media/audio?path=%2Fmnt%2Fhdd0%2Fmedia%2Fmovie.mkv&type=raw&stream=0%3A1&ext=aac" \
  -o movie.aac
curl "http://localhost:8090/media/audio?path=%2Fmnt%2Fhdd0%2Fmedia%2Fmovie.mkv&type=wav&stream=0%3A1" \
  -o movie.wav
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

### `GET /media/tv/files`

Lists all files recursively under an existing TV show folder in the selected **TV** Jellyfin library.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `title` | yes | Exact show folder name (a direct child of the selected library path). |
| `lib-index` | no | Which `jellyfin_tv` path to use (default `0`). |

```bash
curl "http://localhost:8090/media/tv/files?title=Breaking%20Bad%20(2008)"
```

Response:
```json
["/mnt/hdd0/jellyfin/tv/Breaking Bad (2008)/Season 1/Breaking Bad (2008) - S01E01.mkv", "/mnt/hdd0/jellyfin/tv/Breaking Bad (2008)/Season 1/Breaking Bad (2008) - S01E02.mkv"]
```

---

### `GET /media/movies/files`

Lists all files recursively under an existing movie folder in the selected **movies** Jellyfin library.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `title` | yes | Exact movie folder name (a direct child of the selected library path). |
| `lib-index` | no | Which `jellyfin_movies` path to use (default `0`). |

```bash
curl "http://localhost:8090/media/movies/files?title=Inception%20(2010)"
```

Response:
```json
["/mnt/hdd0/jellyfin/movies/Inception (2010)/Inception (2010).mkv", "/mnt/hdd0/jellyfin/movies/Inception (2010)/Inception (2010).en.srt"]
```

---

### `GET /media/tv/file`

Fetches one file from the selected **TV** Jellyfin library for inspection. The `path` value should be an absolute path returned by `GET /media/tv/files` and must refer to a file under the selected `jellyfin_tv` root.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | yes | Absolute file path under the selected TV library. |
| `lib-index` | no | Which `jellyfin_tv` path to use (default `0`). |

The response body is the raw file contents.

```bash
curl "http://localhost:8090/media/tv/file?path=%2Fmnt%2Fhdd0%2Fjellyfin%2Ftv%2FBreaking%20Bad%20(2008)%2FSeason%201%2FBreaking%20Bad%20(2008)%20-%20S01E01.mkv" \
  -o episode-1.mkv
```

---

### `GET /media/movies/file`

Fetches one file from the selected **movies** Jellyfin library for inspection. The `path` value should be an absolute path returned by `GET /media/movies/files` and must refer to a file under the selected `jellyfin_movies` root.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | yes | Absolute file path under the selected movies library. |
| `lib-index` | no | Which `jellyfin_movies` path to use (default `0`). |

The response body is the raw file contents.

```bash
curl "http://localhost:8090/media/movies/file?path=%2Fmnt%2Fhdd0%2Fjellyfin%2Fmovies%2FInception%20(2010)%2FInception%20(2010).mkv" \
  -o Inception.mkv
```

---

### `GET /media/tv/ffprobe`

Returns `ffprobe` metadata for one file under the selected **TV** Jellyfin library as JSON. The `path` value should be an absolute path returned by `GET /media/tv/files`.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | yes | Absolute file path under the selected TV library. |
| `lib-index` | no | Which `jellyfin_tv` path to use (default `0`). |

```bash
curl "http://localhost:8090/media/tv/ffprobe?path=%2Fmnt%2Fhdd0%2Fjellyfin%2Ftv%2FBreaking%20Bad%20(2008)%2FSeason%201%2FBreaking%20Bad%20(2008)%20-%20S01E01.mkv"
```

---

### `GET /media/movies/ffprobe`

Returns `ffprobe` metadata for one file under the selected **movies** Jellyfin library as JSON. The `path` value should be an absolute path returned by `GET /media/movies/files`.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | yes | Absolute file path under the selected movies library. |
| `lib-index` | no | Which `jellyfin_movies` path to use (default `0`). |

```bash
curl "http://localhost:8090/media/movies/ffprobe?path=%2Fmnt%2Fhdd0%2Fjellyfin%2Fmovies%2FInception%20(2010)%2FInception%20(2010).mkv"
```

---

### `GET /media/tv/audio`

Extracts one audio stream from a video file under the selected **TV** Jellyfin library. It uses the same `path`, `type`, `stream`, and raw-only `ext` parameters as `GET /media/audio`; `path` must be under the selected `jellyfin_tv` root.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | yes | Absolute video file path under the selected TV library. |
| `type` | yes | `raw` or `wav`. |
| `stream` | yes | ffmpeg stream map, such as `0:1` or `0:a:1`. |
| `ext` | raw only | Output extension: `aac`, `ac3`, or `m4a`. |
| `lib-index` | no | Which `jellyfin_tv` path to use (default `0`). |

The extracted audio is streamed directly to the response.

The exact ffmpeg commands used are listed in [Audio Extraction Commands](#audio-extraction-commands).

---

### `GET /media/movies/audio`

Extracts one audio stream from a video file under the selected **movies** Jellyfin library. It uses the same `path`, `type`, `stream`, and raw-only `ext` parameters as `GET /media/audio`; `path` must be under the selected `jellyfin_movies` root.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `path` | yes | Absolute video file path under the selected movies library. |
| `type` | yes | `raw` or `wav`. |
| `stream` | yes | ffmpeg stream map, such as `0:1` or `0:a:1`. |
| `ext` | raw only | Output extension: `aac`, `ac3`, or `m4a`. |
| `lib-index` | no | Which `jellyfin_movies` path to use (default `0`). |

The extracted audio is streamed directly to the response.

The exact ffmpeg commands used are listed in [Audio Extraction Commands](#audio-extraction-commands).

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

### `PUT /media/artists/{artist}/delist`

Removes the selected artist folder from the music Jellyfin library. Files under the
media staging directory are not deleted.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `lib-index` | no   | Which `jellyfin_music` path to use (default `0`). |

```bash
curl -X PUT -H "X-Jellybrarian-Token: $JELLYBRARIAN_TOKEN" http://localhost:8090/media/artists/Radiohead/delist
```

Response:
```json
{
  "artist": "Radiohead"
}
```

If the artist folder is already absent, the request succeeds.

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

The optional request body can provide an explicit JSON `files` map instead of automatic video discovery:

```json
{
  "files": {
    "episode-1.mkv": "Season 1/Breaking Bad (2008) - S01E01.mkv",
    "episode-2.mkv": "Season 1/Breaking Bad (2008) - S01E02.mkv"
  }
}
```

When provided, each source path is relative to `media-path` and each destination path is relative to the TV library folder named by `title`. Paths may not escape those directories. If omitted or empty, video files are discovered automatically and filenames are parsed for season/episode information.

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

The optional request body can provide an explicit JSON `files` map instead of automatic file discovery:

```json
{
  "files": {
    "movie.mkv": "Inception (2010).mkv",
    "audio.es.aac": "Inception (2010).es.aac"
  }
}
```

When provided, each source path is relative to `media-path` and each destination path is relative to the movie library folder named by `title`. Paths may not escape those directories. If omitted or empty, video files and supported audio/subtitle tracks are discovered automatically.

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

### `PUT /media/movies/subtitles`

Writes a subtitle file into an existing movie folder in the selected **movies** Jellyfin library.
The request body is the raw subtitle file contents (e.g. **SRT**), not JSON.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `title`   | yes      | Exact movie folder name (same as for `PUT /media/movies/add`, e.g. `Inception (2010)`). |
| `lang`    | no       | If set (e.g. `en`, `es`), the file is named `{title}.{lang}.srt`. If omitted, the file is `{title}.srt`. |
| `lib-index` | no   | Which `jellyfin_movies` path to use (default `0`). |

```bash
curl -X PUT "http://localhost:8090/media/movies/subtitles?title=Inception%20(2010)&lang=en" \
  --data-binary @subtitle.srt
```

Response:
```json
{
  "title": "Inception (2010)",
  "lang": "en",
  "path": "/mnt/hdd0/jellyfin/movies/Inception (2010)/Inception (2010).en.srt"
}
```

**400** if the movie folder does not exist, or if `title` is not a valid single folder name under the library. Files under the **media** directory are not read or modified.

---

### `PUT /media/tv/delist`

Removes the TV show folder `{title}` under the selected **TV** Jellyfin library (everything Jellyfin had for that show: hard links, season folders, etc.). Files under the **media** directory are **not** deleted.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `title`   | yes      | Exact show folder name (as listed by `GET /media/tv/titles`). |
| `lib-index` | no   | Which `jellyfin_tv` path to use (default `0`). |

If that folder is already absent, the request still succeeds (idempotent).

```bash
curl -X PUT "http://localhost:8090/media/tv/delist?title=Breaking%20Bad%20(2008)"
```

Response:
```json
{ "title": "Breaking Bad (2008)" }
```

**400** if `title` cannot be resolved to a direct child folder of the library (e.g. path-like values).

---

### `PUT /media/movies/delist`

Removes the movie folder `{title}` under the selected **movies** Jellyfin library. Same behavior as TV delist: only the library tree is removed; **media** files stay on disk.

**Query parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `title`   | yes      | Exact movie folder name (as listed by `GET /media/movies/titles`). |
| `lib-index` | no   | Which `jellyfin_movies` path to use (default `0`). |

If the folder is already gone, the request succeeds.

```bash
curl -X PUT "http://localhost:8090/media/movies/delist?title=Inception%20(2010)"
```

Response:
```json
{ "title": "Inception (2010)" }
```

**400** if `title` is not a valid single folder name under the library.

---

## Audio Extraction Commands

The audio endpoints execute ffmpeg directly, without a shell. `$path` is the validated absolute input path and `$stream` is the requested stream map. Output is always written to stdout through `pipe:1`; `out` is not a request parameter and no output file is created on the server.

For `type=raw&ext=aac`:
```text
ffmpeg -nostdin -v error -i "$path" -map "$stream" -c:a copy -f adts pipe:1
```

For `type=raw&ext=ac3`:
```text
ffmpeg -nostdin -v error -i "$path" -map "$stream" -c:a copy -f ac3 pipe:1
```

For `type=raw&ext=m4a`:
```text
ffmpeg -nostdin -v error -i "$path" -map "$stream" -c:a copy -f mp4 -movflags frag_keyframe+empty_moov+default_base_moof -avoid_negative_ts make_zero pipe:1
```

The fragmented MP4 flags are required because stdout is not seekable.

For `type=wav`:
```text
ffmpeg -nostdin -v error -i "$path" -map "$stream" -ac 1 -ar 22050 -f wav pipe:1
```

These commands are identical for `GET /media/audio`, `GET /media/tv/audio`, and `GET /media/movies/audio`; only the validated root for `$path` differs.

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
├── ffmpeg/
│   └── ffmpeg.go        # ffmpeg and ffprobe subprocess operations
├── server/
│   ├── server.go        # HTTP route definitions
│   ├── library.go       # LibraryKind, createMediaManager, lib-index resolution
│   └── input.go         # query parameter helpers
├── Dockerfile
└── config.toml
```
