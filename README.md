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

Configuration is loaded with the following precedence (highest to lowest):

1. **CLI flags**
2. **Environment variables** (`PLEXISHOW_*`)
3. **YAML config file**
4. **Built-in defaults**

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
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `PLEXISHOW_M3U_URL` | M3U playlist URL |
| `PLEXISHOW_EPG_URL` | EPG XMLTV URL |
| `PLEXISHOW_LISTEN_ADDR` | HTTP listen address (default `:8080`) |
| `PLEXISHOW_MAX_STREAMS` | Max concurrent streams (default `4`) |
| `PLEXISHOW_STREAM_TIMEOUT` | Per-stream idle timeout (default `30s`) |
| `PLEXISHOW_REFRESH_INTERVAL` | M3U refresh interval (default `1h`) |
| `PLEXISHOW_FFMPEG_PATH` | Path to ffmpeg binary (default `ffmpeg`) |
| `PLEXISHOW_DEFAULT_HEADERS_TOKEN` | Default X-TCDN-token header |
| `PLEXISHOW_DEFAULT_HEADERS_REFERER` | Default Referer header |
| `PLEXISHOW_DEFAULT_HEADERS_USER_AGENT` | Default User-Agent header |

### CLI Flags

```
-config string
    Path to config file (default "config.yaml")
-m3u-url string
    M3U playlist URL (overrides config/env)
-epg-url string
    EPG XMLTV URL (overrides config/env)
-listen-addr string
    HTTP listen address (overrides config/env)
-max-streams int
    Max concurrent streams (overrides config/env)
-stream-timeout string
    Per-stream idle timeout (overrides config/env)
-refresh-interval string
    M3U refresh interval (overrides config/env)
-ffmpeg-path string
    Path to ffmpeg binary (overrides config/env)
-token string
    X-TCDN-token header value (overrides config/env)
-referer string
    Referer header value (overrides config/env)
-user-agent string
    User-Agent header value (overrides config/env)
```

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
# With a config file
./plexishow -config config.yaml

# With environment variables
export PLEXISHOW_M3U_URL="https://example.com/playlist.m3u"
export PLEXISHOW_EPG_URL="https://example.com/epg.xml"
./plexishow

# With CLI flags only
./plexishow -m3u-url "https://example.com/playlist.m3u" -epg-url "https://example.com/epg.xml"
```

### Docker

```bash
# Build image
mage docker
```

### Docker GPU (VAAPI)

For Intel/AMD GPU hardware-accelerated decoding via VAAPI:

```bash
# Build GPU image
mage dockergpu
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
# Run tests (inside Dagger)
mage test

# Run tests with coverage gate (inside Dagger)
mage cover

# Run linter (inside Dagger)
mage lint

# Build binary (inside Dagger)
mage build

# Build Docker image (inside Dagger)
mage docker

# Build GPU Docker image (inside Dagger)
mage dockerGPU

# Run linter
mage vet

# Format code
mage fmt

# Clean build artifacts
mage clean

# Generate SBOM (inside Dagger)
mage sbom

# Scan for vulnerabilities (inside Dagger)
mage vulnscan
```

### Dagger Build System

Builds and tests run inside **Dagger** containers for reproducibility.
The magefile uses **cache volumes** for Go module and build caches:

- `go-mod-cache` — Go module download cache
- `go-build-cache` — Go build/compilation cache

Docker image builds push a **build cache** to `ghcr.io/segator/plexishow:buildcache`,
which speeds up subsequent builds by reusing layers.

No external services required — everything runs locally or in CI.

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
