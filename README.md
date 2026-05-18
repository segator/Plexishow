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

#### Server

| Variable | Requirement | Description |
|----------|-------------|-------------|
| `PLEXISHOW_M3U_URL` | **Mandatory** | M3U playlist URL |
| `PLEXISHOW_EPG_URL` | Optional | EPG XMLTV URL |
| `PLEXISHOW_BASE_URL` | Optional | Base URL advertised to clients |
| `PLEXISHOW_LISTEN_ADDR` | Optional | HTTP listen address (default `:8080`) |
| `PLEXISHOW_MAX_STREAMS` | Optional | Max concurrent streams (default `4`) |
| `PLEXISHOW_STREAM_TIMEOUT` | Optional | Per-stream idle timeout (default `30s`) |
| `PLEXISHOW_REFRESH_INTERVAL` | Optional | M3U refresh interval (default `5m`) |
| `PLEXISHOW_FFMPEG_PATH` | Optional | Path to ffmpeg binary (default `ffmpeg`) |
| `PLEXISHOW_LOGS_DIR` | Optional | Per-channel ffmpeg log directory (default `/tmp/plexishow-logs`) |
| `PLEXISHOW_TRANSCODE` | Optional | Enable full CPU transcoding (default `false`) |

#### FFmpeg

| Variable | Requirement | Description |
|----------|-------------|-------------|
| `PLEXISHOW_FFMPEG_PROBESIZE` | Optional | Probe buffer size in bytes (default `"500000"`) |
| `PLEXISHOW_FFMPEG_ANALYZEDURATION` | Optional | Maximum analysis duration in µs (default `"500000"`) |
| `PLEXISHOW_FFMPEG_TRANSCODE` | Optional | Enable real-time transcoding (default `false`) |
| `PLEXISHOW_FFMPEG_HWACCEL` | Optional | GPU hardware accelerator (`nvenc`, `vaapi`, `qsv`) |
| `PLEXISHOW_FFMPEG_PRESET` | Optional | Encoder preset (default `"veryfast"`) |
| `PLEXISHOW_FFMPEG_CRF` | Optional | CRF quality parameter (default `18`) |
| `PLEXISHOW_FFMPEG_AUDIO_CODEC` | Optional | Audio encoder codec (default `"aac"`) |
| `PLEXISHOW_FFMPEG_AUDIO_BITRATE` | Optional | Transcoded audio stream bitrate (default `"192k"`) |
| `PLEXISHOW_FFMPEG_VAAPI_DEVICE` | Optional | VAAPI graphics device path (default `"/dev/dri/renderD128"`) |
| `PLEXISHOW_FFMPEG_RECONNECT` | Optional | Auto-reconnect HTTP socket on network failures (default `true`) |
| `PLEXISHOW_FFMPEG_RECONNECT_STREAMED` | Optional | Auto-reconnect live streamed HTTP payloads (default `true`) |
| `PLEXISHOW_FFMPEG_RECONNECT_DELAY_MAX` | Optional | Max delay in seconds between reconnect tries (default `5`) |
| `PLEXISHOW_FFMPEG_RW_TIMEOUT` | Optional | Read-write timeout in microseconds (default `"10000000"`) |

#### Default Headers

| Variable | Requirement | Description |
|----------|-------------|-------------|
| `PLEXISHOW_DEFAULT_HEADERS_TOKEN` | Optional | X-TCDN-token for all channels |

### CLI Flags

#### Server

- **`-config`** *(Optional)*: Path to config file (default `"config.yaml"`).
- **`-m3u-url`** *(Mandatory)*: M3U playlist URL.
- **`-epg-url`** *(Optional)*: EPG XMLTV URL.
- **`-base-url`** *(Optional)*: Base URL advertised to clients.
- **`-listen-addr`** *(Optional)*: HTTP listen address (default `:8080`).
- **`-max-streams`** *(Optional)*: Max concurrent streams (default `4`).
- **`-stream-timeout`** *(Optional)*: Per-stream idle timeout (default `30s`).
- **`-refresh-interval`** *(Optional)*: M3U refresh interval (default `5m`).
- **`-token`** *(Optional)*: X-TCDN-token for all channels.
- **`-logs-dir`** *(Optional)*: Per-channel ffmpeg log directory.
- **`-transcode`** *(Optional)*: Enable full CPU transcoding (default `false`).

#### FFmpeg

- **`-ffmpeg-probesize`** *(Optional)*: Probe buffer size in bytes.
- **`-ffmpeg-analyzeduration`** *(Optional)*: Maximum analysis duration in µs.
- **`-ffmpeg-transcode`** *(Optional)*: Enable real-time transcoding (default `false`).
- **`-ffmpeg-hwaccel`** *(Optional)*: GPU acceleration engine (`nvenc`, `vaapi`, `qsv`).
- **`-ffmpeg-preset`** *(Optional)*: Encoder preset (default `"veryfast"`).
- **`-ffmpeg-crf`** *(Optional)*: CRF quality parameter (default `18`).
- **`-ffmpeg-audio-codec`** *(Optional)*: Audio encoder codec (default `"aac"`).
- **`-ffmpeg-audio-bitrate`** *(Optional)*: Transcoded audio bitrate (default `"192k"`).
- **`-ffmpeg-vaapi-device`** *(Optional)*: VAAPI device path (default `"/dev/dri/renderD128"`).
- **`-ffmpeg-reconnect`** *(Optional)*: Auto-reconnect on HTTP failures (default `true`).
- **`-ffmpeg-reconnect-streamed`** *(Optional)*: Auto-reconnect live HTTP streams (default `true`).
- **`-ffmpeg-reconnect-delay-max`** *(Optional)*: Max reconnect delay in seconds (default `5`).
- **`-ffmpeg-rw-timeout`** *(Optional)*: Read/write timeout in microseconds (default `"10000000"`).

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

Plexishow is packaged and published as an **OCI Helm Chart** in the GitHub Container Registry (`ghcr.io`).

### Production Installation (OCI Registry)

```bash
# Log in to GHCR Helm Registry (if required)
helm registry login ghcr.io -u <your-username>

# Install the official release from GHCR
helm install plexishow oci://ghcr.io/segator/plexishow \
  --version 1.0.0 \
  --set config.m3u_url="https://example.com/playlist.m3u" \
  --set config.epg_url="https://example.com/epg.xml"
```

To install the latest development version built from the `main` branch:

```bash
helm install plexishow oci://ghcr.io/segator/plexishow \
  --version 0.0.0-dev \
  --set config.m3u_url="https://example.com/playlist.m3u" \
  --set config.epg_url="https://example.com/epg.xml"
```

### Local Development / Source Installation

You can also install the chart directly from the source repository:

```bash
helm install plexishow ./helm/plexishow \
  --set config.m3u_url="https://example.com/playlist.m3u" \
  --set config.epg_url="https://example.com/epg.xml"
```

### Key Chart Features

- **Dynamic Configuration Map:** The `config:` block in `values.yaml` is fully serialized into `/etc/plexishow/config.yaml` using `toYaml`. This means all configuration capabilities supported by Plexishow (like advanced `ffmpeg` or hardware transcoding parameters) can be customized dynamically without altering Helm templates.
- **Gateway API Support:** Includes native support for exposing the service using the Kubernetes Gateway API. Simply enable `httproute.enabled` and configure your parent Gateway refs in `values.yaml`.

See [values.yaml](file:///home/aymerici/Plexishow/helm/plexishow/values.yaml) for all available options and detailed comments.

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
