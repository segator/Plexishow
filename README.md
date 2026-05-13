# Plexishow

Plexishow is an IPTV decryption proxy for Plex. It fetches encrypted M3U playlists, parses per-channel ClearKey metadata, and exposes **HDHomeRun-compatible endpoints** so Plex DVR can consume them natively. It serves decrypted MPEG-TS streams by spawning `ffmpeg` on demand.

---

## Features

- **HDHomeRun API** — Emulates HDHomeRun discover and lineup endpoints for seamless Plex integration
- **Clean M3U** — Exposes a sanitized `/channels.m3u` playlist
- **XMLTV EPG** — Proxies and serves `/epg.xml` for channel guide data
- **ClearKey Decryption** — Automatically injects per-channel decryption keys into ffmpeg
- **Concurrent Stream Limits** — Configurable max streams with semaphore-based backpressure
- **Graceful Shutdown** — Cleans up active ffmpeg processes on SIGINT/SIGTERM
- **Prometheus Metrics** — Exposes active stream count and error counters
- **Health Endpoint** — Simple `/health` check for load balancers and orchestrators

---

## Architecture

```
┌─────────────┐     ┌─────────┐     ┌───────┐     ┌─────────────────────────────────┐
│  M3U Source │────▶│ Parser  │────▶│ Store │────▶│  HDHR API  │───▶  Plex           │
│  (encrypted)│     │(ClearKey│     │       │     │  /discover.json                  │
│             │     │ metadata│     │       │     │  /lineup.json                    │
└─────────────┘     └─────────┘     └───────┘     │  /channels.m3u                   │
                                                  │  /epg.xml                        │
                                                  │  /stream/{id}  ──▶  ffmpeg  ──▶  │
                                                  │  /health                         │
                                                  │  /metrics                        │
                                                  └─────────────────────────────────┘
```

1. **M3U Source** — Encrypted playlist fetched on startup and refreshed periodically
2. **Parser** — Extracts channel URLs, headers, and per-channel ClearKey credentials
3. **Store** — In-memory channel registry
4. **HDHR API / Stream / EPG** — HTTP handlers that Plex talks to
5. **ffmpeg** — Spawned on-demand to decrypt and remux streams to MPEG-TS

---

## Configuration

All settings are optional. The app starts with built-in defaults and layers configuration from (highest to lowest precedence):

1. **CLI flags**
2. **Environment variables** (`PLEXISHOW_*`)
3. **YAML config file** (only if the file exists — no error if missing)
4. **Built-in defaults**

The only **required** setting at runtime is a valid `m3u_url` (via flag, env, or config file).

The **EPG URL** is optional. If not provided via CLI flag, env, or config file, the app tries to extract it from the `url-tvg` attribute in the `#EXTM3U` header of the playlist. If none is found, it logs a warning and the `/epg.xml` endpoint returns 404.

### YAML Config

```yaml
m3u_url: "https://example.com/playlist.m3u"
epg_url: "https://example.com/epg.xml"
listen_addr: ":8080"
max_streams: 4
stream_timeout: 30s
refresh_interval: 1h
ffmpeg_path: "ffmpeg"
default_headers:
  token: "your-token"
  referer: "https://example.com"
  user_agent: "Plexishow/1.0"

# Advanced FFmpeg Settings (All optional, defaults shown below)
ffmpeg:
  transcode: false                # Enable real-time transcoding instead of direct copy
  hwaccel: ""                     # Hardware acceleration engine: "" (CPU), "nvenc", "vaapi", "qsv"
  preset: "veryfast"              # Encoder preset (e.g., veryfast, ultrafast, or p4 for NVENC)
  crf: 18                         # Constant Rate Factor / quality parameter (18-23 is recommended)
  audio_bitrate: "192k"           # Audio bitrate (e.g., 128k, 192k)
  vaapi_device: "/dev/dri/renderD128" # Path to your VAAPI rendering device (AMD/Intel)
  reconnect: true                 # Automatically reconnect to stream on network drops
  reconnect_streamed: true        # Automatically reconnect live HTTP feeds (DASH/HLS)
  reconnect_delay_max: 5          # Maximum delay in seconds before retrying reconnect
  rw_timeout: "10000000"          # Read/write socket timeout in microseconds (10s)
  probesize: "1500000"            # Probe buffer size in bytes for analyzing stream
  analyzeduration: "1000000"      # Maximum time in microseconds to analyze stream
```

### Environment Variables

| Variable | Requirement | Description |
|----------|-------------|-------------|
| `PLEXISHOW_M3U_URL` | **Mandatory** | M3U playlist URL |
| `PLEXISHOW_EPG_URL` | Optional | EPG XMLTV URL |
| `PLEXISHOW_LISTEN_ADDR` | Optional | HTTP listen address (default `:8080`) |
| `PLEXISHOW_MAX_STREAMS` | Optional | Max concurrent streams (default `4`) |
| `PLEXISHOW_STREAM_TIMEOUT` | Optional | Per-stream idle timeout (default `30s`) |
| `PLEXISHOW_REFRESH_INTERVAL`| Optional | M3U refresh interval (default `1h`) |
| `PLEXISHOW_FFMPEG_PATH` | Optional | Path to ffmpeg binary (default `ffmpeg`) |
| `PLEXISHOW_BASE_URL` | Optional | Base URL advertised to clients |
| `PLEXISHOW_TOKEN` | Optional | X-TCDN-token for all channels |
| `PLEXISHOW_FFMPEG_TRANSCODE` | Optional | Enable full real-time transcoding (default `false`) |
| `PLEXISHOW_FFMPEG_HWACCEL` | Optional | GPU hardware accelerator (`nvenc`, `vaapi`, `qsv`, default `""`) |
| `PLEXISHOW_FFMPEG_PRESET` | Optional | Codifier preset (default `"veryfast"`) |
| `PLEXISHOW_FFMPEG_CRF` | Optional | CRF quality parameter (default `18`) |
| `PLEXISHOW_FFMPEG_AUDIO_BITRATE`| Optional| Transcoded audio stream bitrate (default `"192k"`) |
| `PLEXISHOW_FFMPEG_VAAPI_DEVICE` | Optional | AMD/Intel VAAPI graphics device path (default `"/dev/dri/renderD128"`) |
| `PLEXISHOW_FFMPEG_RECONNECT` | Optional | Automatically reconnect HTTP socket on network failures (default `true`) |
| `PLEXISHOW_FFMPEG_RECONNECT_STREAMED` | Optional | Auto-reconnect live streamed HTTP payloads (default `true`) |
| `PLEXISHOW_FFMPEG_RECONNECT_DELAY_MAX` | Optional | Maximum delay in seconds between reconnect tries (default `5`) |
| `PLEXISHOW_FFMPEG_RW_TIMEOUT` | Optional | Read-write timeout threshold in microseconds (default `"10000000"`) |
| `PLEXISHOW_FFMPEG_PROBESIZE` | Optional | Analysis buffer probing size in bytes (default `"1500000"`) |
| `PLEXISHOW_FFMPEG_ANALYZE_DURATION` | Optional | Maximum analysis duration in microseconds (default `"1000000"`) |

### CLI Flags

- **`-m3u-url`** *(Mandatory)*: M3U playlist URL (overrides config/env). This is the only required parameter.
- **`-config`** *(Optional)*: Path to config file (default "config.yaml").
- **`-epg-url`** *(Optional)*: EPG XMLTV URL (overrides config/env).
- **`-base-url`** *(Optional)*: Base URL advertised to clients (overrides config/env).
- **`-listen-addr`** *(Optional)*: HTTP listen address (default ":8080", overrides config/env).
- **`-max-streams`** *(Optional)*: Max concurrent streams (overrides config/env).
- **`-stream-timeout`** *(Optional)*: Per-stream idle timeout (overrides config/env).
- **`-refresh-interval`** *(Optional)*: M3U refresh interval (overrides config/env).
- **`-token`** *(Optional)*: X-TCDN-token for all channels (overrides M3U stream_headers).
- **`-ffmpeg-transcode`** *(Optional)*: Enable transcoding (overrides config/env, default `false`).
- **`-ffmpeg-hwaccel`** *(Optional)*: Set GPU acceleration: `nvenc`, `vaapi`, `qsv` (overrides config/env).
- **`-ffmpeg-preset`** *(Optional)*: Codec speed preset (overrides config/env).
- **`-ffmpeg-crf`** *(Optional)*: Quality/CRF level (overrides config/env).
- **`-ffmpeg-audio-bitrate`** *(Optional)*: Trascoded audio bit rate (overrides config/env).
- **`-ffmpeg-vaapi-device`** *(Optional)*: VAAPI hardware driver device (overrides config/env).
- **`-ffmpeg-reconnect`** *(Optional)*: Toggle HTTP autoconnect (default `true`, overrides config/env).
- **`-ffmpeg-reconnect-streamed`** *(Optional)*: Toggle live HTTP stream autoconnect (default `true`, overrides config/env).
- **`-ffmpeg-reconnect-delay-max`** *(Optional)*: Max delay in seconds for reconnection (overrides config/env).
- **`-ffmpeg-rw-timeout`** *(Optional)*: R/W timeout in microseconds (overrides config/env).
- **`-ffmpeg-probesize`** *(Optional)*: Probesize in bytes (overrides config/env).
- **`-ffmpeg-analyzeduration`** *(Optional)*: Analyzeduration in microseconds (overrides config/env).

---

## Installation

### Build from source

```bash
mage build
```

The binary is written to `bin/plexishow`.

### Download a release

Pre-built binaries for Linux, macOS, and Windows (amd64 / arm64) are available on the [Releases](https://github.com/segator/Plexishow/releases) page.

---

## Usage

### Binary

```bash
# With CLI flags only (no config file needed)
./plexishow -m3u-url "https://example.com/playlist.m3u" -epg-url "https://example.com/epg.xml"

# With environment variables
export PLEXISHOW_M3U_URL="https://example.com/playlist.m3u"
export PLEXISHOW_EPG_URL="https://example.com/epg.xml"
./plexishow

# With a config file
./plexishow -config config.yaml

# Or simply run via mage (no config file required)
mage run

# Pass flags to the binary via mage (use --)
mage run -- -help
mage run -- -m3u-url "https://example.com/playlist.m3u"
```

### Docker Run

The Docker image is published to `ghcr.io/segator/plexishow`.

```bash
docker run -d --name plexishow \
  -p 8080:8080 \
  -e PLEXISHOW_M3U_URL="https://example.com/playlist.m3u" \
  ghcr.io/segator/plexishow:latest
```

### Docker Compose

Create a `docker-compose.yml`:

```yaml
version: '3.8'
services:
  plexishow:
    image: ghcr.io/segator/plexishow:latest
    container_name: plexishow
    ports:
      - "8080:8080"
    environment:
      - PLEXISHOW_M3U_URL=https://example.com/playlist.m3u
      - PLEXISHOW_EPG_URL=https://example.com/epg.xml
    restart: unless-stopped
```

Then run:

```bash
docker-compose up -d
```

---

## Kubernetes / Helm

A Helm chart is included in `helm/plexishow`.

```bash
helm install plexishow ./helm/plexishow \
  --set config.m3u_url="https://example.com/playlist.m3u" \
  --set config.epg_url="https://example.com/epg.xml"
```

See `helm/plexishow/values.yaml` for all available options.

---

## Metrics

Plexishow exposes Prometheus-compatible metrics at `/metrics`.

### Application Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `plexishow_active_streams` | gauge | Current active ffmpeg streams |
| `plexishow_stream_errors_total` | counter | Total stream errors |
| `plexishow_channel_viewers` | gauge | Current viewers per channel (labeled by `channel`) |
| `plexishow_channel_bytes_sent_total` | counter | Total egress bytes served to clients per channel (labeled by `channel`) |
| `plexishow_m3u_channels_total` | gauge | Total channels successfully parsed and loaded from the M3U playlist |
| `plexishow_m3u_last_refresh_timestamp_seconds` | gauge | Unix epoch timestamp of the last successful M3U refresh |
| `plexishow_epg_last_refresh_timestamp_seconds` | gauge | Unix epoch timestamp of the last successful EPG guide refresh |

### Go Runtime Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `go_goroutines` | gauge | Number of goroutines |
| `go_memstats_alloc_bytes_bytes` | gauge | Bytes allocated and in use |
| `go_memstats_sys_bytes` | gauge | Total bytes obtained from system |
| `go_memstats_heap_alloc_bytes` | gauge | Heap allocation bytes |
| `go_memstats_heap_inuse_bytes` | gauge | Heap in-use bytes |
| `go_memstats_heap_objects` | gauge | Number of allocated heap objects |
| `go_memstats_gc_cpu_fraction` | gauge | Fraction of CPU time used by GC |
| `go_memstats_last_gc_time_seconds` | gauge | Unix timestamp of last GC |
| `go_threads` | gauge | Number of OS threads |

---

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /discover.json` | HDHomeRun device discovery |
| `GET /lineup.json` | HDHomeRun channel lineup |
| `GET /channels.m3u` | Clean M3U playlist |
| `GET /epg.xml` | XMLTV EPG data |
| `GET /stream/{id}` | Decrypted MPEG-TS stream |
| `GET /health` | Health check (`200 OK`) |
| `GET /metrics` | Prometheus metrics |

---

## Development

```bash
# Run tests (race + coverage gate)
mage test

# Run linter
mage lint

# Build binary
mage bin:build

# Run binary locally
mage run
mage run -- -help

# Build Docker image
mage docker:build

# Publish Docker images
mage docker:publish

# Format code
mage fmt

# Vet code
mage vet

# Clean build artifacts
mage clean

# Generate SBOM
mage sbom

# Security scan (govulncheck + SBOM + Grype)
mage security

# Release snapshot
mage releaseSnapshot

# Full pipeline
mage all
```

---

## Release

This project uses [Release Please](https://github.com/googleapis/release-please) and [GoReleaser](https://goreleaser.com/) for automated releases.

### Release process

1. **Conventional Commits** — Write commit messages following the [Conventional Commits](https://www.conventionalcommits.org/) specification (e.g., `feat:`, `fix:`, `chore:`).
2. **Release PR** — On every push to `main`, `release-please` analyzes commits and opens (or updates) a release PR with a changelog and version bump.
3. **Merge Release PR** — Merging the release PR creates a GitHub release and tags the commit (e.g., `v1.2.3`).
4. **Release Workflow** — Pushing a `v*` tag triggers the `release.yaml` workflow, which runs GoReleaser to build and publish binaries and container images.

### Local snapshot

```bash
mage releasesnapshot
```

### Manual release

```bash
mage release
```

---



## License

MIT
