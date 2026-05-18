# Plexishow - Agent Guidelines

Welcome, Agent! This document (`agents.md`) provides context, architectural rules, and conventions for working on the **Plexishow** codebase. Please review these guidelines before making modifications.

## 🎯 Project Overview

**Plexishow** is an IPTV decryption proxy for Plex. 
It fetches encrypted M3U playlists, parses per-channel ClearKey metadata, and exposes **HDHomeRun-compatible endpoints** so Plex DVR can consume them natively. It serves decrypted MPEG-TS streams by spawning `ffmpeg` on demand.

## 🏗️ Architecture

The application is written in Go (1.22+) and uses a clean, package-oriented structure:

- **`cmd/plexishow/`**: The main entrypoint. Handles flag parsing, wiring dependencies, and running the HTTP server.
- **`internal/config/`**: Unified configuration loading supporting YAML files, environment variables (`PLEXISHOW_*`), and CLI flags. Precedence: Flags > Env > File > Defaults.
- **`internal/m3u/`**: M3U parser that extracts EXTINF, KODIPROP, EXTVLCOPT, and ClearKey metadata into a standard `Channel` model.
- **`internal/store/`**: An in-memory, thread-safe registry holding parsed channels.
- **`internal/hdhr/`**: HDHomeRun API implementation (serves `/discover.json`, `/lineup.json`, `/device.xml`).
- **`internal/epg/`**: XMLTV proxy and server (serves `/epg.xml`).
- **`internal/stream/`**: The streaming engine. Manages `ffmpeg` lifecycles, concurrency limits (semaphore-based), timeouts, and stream proxying.
- **`internal/server/`**: HTTP routing using the standard library `http.ServeMux`.
- **`internal/metrics/`**: Prometheus metrics integration (`/metrics`).
- **`test/fixtures/`**: Contains static files used for testing (e.g., sample M3U playlists).

## 🛠️ Coding Conventions & Principles

1. **No Global State**: 
   Avoid global variables. Pass dependencies explicitly via struct fields and constructors (e.g., pass the `Store` to the `Server` and `HDHR Handler`).
   
2. **Standard Library First**: 
   Rely on the Go standard library as much as possible. For routing, use `net/http` `ServeMux`. Keep external dependencies to an absolute minimum (currently using `gopkg.in/yaml.v3` for config parsing).

3. **Graceful Shutdown**: 
   Always handle `context.Context` properly. Ensure that active `ffmpeg` processes are cleanly terminated on `SIGINT`/`SIGTERM` and that the HTTP server shuts down gracefully.

4. **Testing**: 
   Write unit tests alongside the code (`*_test.go`). Use the fixtures in `test/fixtures/` when testing parsers. Ensure edge cases are covered. 
   To run tests locally, use `mage test`.

5. **Error Handling**: 
   Don't ignore errors. Wrap errors with context where appropriate using `fmt.Errorf("do something: %w", err)`.

6. **Interfaces for IO**: 
   For boundaries that interact with external services or file systems, use interfaces to make the components easily mockable and testable.

## ⚙️ Build and Development Workflow

The project uses [Mage](https://magefile.org/) as its build tool instead of `make`.

Here are the key commands to use:
- `mage build` or `mage bin:build`: Build the Go binary (output in `bin/`).
- `mage test`: Run all tests with the race detector enabled.
- `mage lint`: Run `golangci-lint` to check code quality.
- `mage run`: Run the application locally.
- `mage fmt`: Format the code.

## 🤖 Modifying Code

When asked to implement a new feature or fix a bug:
1. Identify the correct `internal/*` package for the domain logic.
2. If modifying configuration, ensure changes are reflected across flags, environment variables, and the YAML struct in `internal/config`. Update the `README.md` if necessary.
3. Write or update tests to verify your changes.
4. Run `mage lint` and `mage test` to ensure CI will pass.

## 🔄 CI / CD

Two workflows run in parallel on every PR and push to `main`:

- **CI** (`.github/workflows/ci.yaml`) — Build, test, and publish pipeline.
- **Security** (`.github/workflows/security.yaml`) — Vulnerability scanning and SARIF uploads.

### CI Pipeline Steps

1. **Format** — `mage fmt` + `git diff --exit-code`
2. **Lint** — `mage lint` (golangci-lint)
3. **Test + Coverage** — `mage test` (race detector + 40% threshold)
4. **Docker** — `mage docker:build`
5. **Publish** — `mage docker:publish` to GHCR (only on `main` pushes)

### Security Pipeline Steps

Runs `mage security` which executes in a single step:
- **govulncheck** — Official Go vulnerability scanner (SARIF)
- **SBOM** — Syft SPDX JSON generation
- **Grype** — Container/dependency vulnerability scan (SARIF + table output, fails on critical)

SARIF reports are uploaded to the GitHub Security tab automatically. The security workflow also runs weekly on Mondays at 03:00 UTC.

### Release

Tag pushes (`v*`) trigger GoReleaser via `.github/workflows/release-please.yaml`.
