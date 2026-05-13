# IPTV Decryption Proxy for Plex — Implementation Plan

> **For Hermes:** Use `subagent-driven-development` skill to implement this plan task-by-task.

**Goal:** Build a Go service that fetches an encrypted IPTV M3U playlist, parses per-channel ClearKey metadata, exposes HDHomeRun-compatible discovery endpoints for Plex, and serves decrypted MPEGTS streams by spawning ffmpeg on demand.

**Architecture:** A single Go binary with clean internal packages: `config` (YAML), `m3u` (parser), `store` (in-memory channel index), `hdhr` (HDHomeRun API), `epg` (XMLTV passthrough/transform), `stream` (ffmpeg lifecycle), and `server` (HTTP routing). All IO-bound boundaries use interfaces for testability. No database, no web UI, no globals.

**Tech Stack:** Go 1.22+, `gopkg.in/yaml.v3`, standard `net/http`, `github.com/gorilla/mux` (or stdlib `ServeMux` if no path variables needed; prefer stdlib to reduce deps), ffmpeg 6.x/7.x, Docker BuildKit with `docker buildx` for multi-arch images.

**Configuration Precedence:** CLI flags > Environment variables > Config file (`config.yaml`) > Hardcoded defaults. All settings can be set via any method.

---

## Table of Contents

1. [Phase 0: Bootstrap & Tooling](#phase-0-bootstrap--tooling)
2. [Phase 1: Configuration & Domain Models](#phase-1-configuration--domain-models)
3. [Phase 2: M3U Parser (with Fixtures)](#phase-2-m3u-parser-with-fixtures)
4. [Phase 3: In-Memory Channel Store](#phase-3-in-memory-channel-store)
5. [Phase 4: Periodic M3U Fetcher](#phase-4-periodic-m3u-fetcher)
6. [Phase 5: HDHomeRun API Surface](#phase-5-hdhomeRun-api-surface)
7. [Phase 6: Clean M3U & XMLTV EPG Endpoints](#phase-6-clean-m3u--xmltv-epg-endpoints)
8. [Phase 7: FFmpeg Streaming Engine](#phase-7-ffmpeg-streaming-engine)
9. [Phase 8: HTTP Server & Graceful Shutdown](#phase-8-http-server--graceful-shutdown)
10. [Phase 9: Observability (Health & Metrics)](#phase-9-observability-health--metrics)
11. [Phase 10: Dockerfile & Multi-Arch Build](#phase-10-dockerfile--multi-arch-build)
12. [Phase 11: Integration & Acceptance](#phase-11-integration--acceptance)

---

## Phase 0: Bootstrap & Tooling

### Task 0.1: Initialize Go Module

**Objective:** Create module root and basic `.gitignore`.

**Files:**
- Create: `go.mod`
- Create: `.gitignore`

**Step 1: Initialize module**

Run:
```bash
cd /home/aymerici/Plexishow
go mod init github.com/aymerici/plexishow
```

**Step 2: Add `.gitignore`**

```gitignore
/bin/
/dist/
*.test
*.out
.env
.idea/
.vscode/
coverage.html
```

**Step 3: Commit**

```bash
git add go.mod .gitignore
git commit -m "chore: init go module"
```

---

### Task 0.2: Define Project Layout

**Objective:** Establish package structure so later tasks have exact paths.

**Files:**
- Create directories:
  - `cmd/plexishow/`
  - `internal/config/`
  - `internal/m3u/`
  - `internal/store/`
  - `internal/hdhr/`
  - `internal/epg/`
  - `internal/stream/`
  - `internal/server/`
  - `internal/metrics/`
  - `test/fixtures/`

Run:
```bash
mkdir -p cmd/plexishow internal/{config,m3u,store,hdhr,epg,stream,server,metrics} test/fixtures
```

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: create package layout"
```

---

## Phase 1: Configuration & Domain Models

### Task 1.1: Define Config Struct with Env & CLI Support

**Objective:** Build a unified config loader that supports YAML, environment variables (prefix `PLEXISHOW_`), and CLI flags with proper precedence: flags > env > file > defaults.

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/env.go`
- Create: `internal/config/flags.go`
- Test: `internal/config/config_test.go`

**Step 1: Write failing tests**

```go
package config

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestLoadFromFile(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "config.yaml")
    data := []byte(`
m3u_url: "https://example.com/playlist.m3u"
epg_url: "https://example.com/epg.xml"
listen_addr: ":8080"
max_streams: 4
stream_timeout: "30s"
refresh_interval: "1h"
ffmpeg_path: "/usr/bin/ffmpeg"
default_headers:
  token: "Bearer abc"
  referer: "https://tv.movistar.com.pe/"
  user_agent: "Mozilla/5.0"
`)
    if err := os.WriteFile(p, data, 0644); err != nil {
        t.Fatal(err)
    }

    cfg, err := Load(p, nil)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if cfg.M3UURL != "https://example.com/playlist.m3u" {
        t.Errorf("m3u_url mismatch")
    }
    if cfg.MaxStreams != 4 {
        t.Errorf("max_streams mismatch")
    }
    if cfg.DefaultHeaders.Token != "Bearer abc" {
        t.Errorf("token mismatch")
    }
}

func TestLoadFromEnvOverridesFile(t *testing.T) {
    os.Setenv("PLEXISHOW_M3U_URL", "http://env.com/m3u")
    os.Setenv("PLEXISHOW_MAX_STREAMS", "8")
    os.Setenv("PLEXISHOW_DEFAULT_HEADERS_TOKEN", "env-token")
    defer os.Unsetenv("PLEXISHOW_M3U_URL")
    defer os.Unsetenv("PLEXISHOW_MAX_STREAMS")
    defer os.Unsetenv("PLEXISHOW_DEFAULT_HEADERS_TOKEN")

    dir := t.TempDir()
    p := filepath.Join(dir, "config.yaml")
    data := []byte(`m3u_url: "https://file.com/playlist.m3u"
max_streams: 2
`)
    os.WriteFile(p, data, 0644)

    cfg, err := Load(p, nil)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.M3UURL != "http://env.com/m3u" {
        t.Errorf("env should override file: got %s", cfg.M3UURL)
    }
    if cfg.MaxStreams != 8 {
        t.Errorf("env should override file: got %d", cfg.MaxStreams)
    }
    if cfg.DefaultHeaders.Token != "env-token" {
        t.Errorf("nested env should work: got %s", cfg.DefaultHeaders.Token)
    }
}

func TestLoadFromFlagsOverridesEnv(t *testing.T) {
    os.Setenv("PLEXISHOW_M3U_URL", "http://env.com/m3u")
    defer os.Unsetenv("PLEXISHOW_M3U_URL")

    flags := map[string]string{"m3u_url": "http://flag.com/m3u"}
    cfg, err := Load("", flags)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.M3UURL != "http://flag.com/m3u" {
        t.Errorf("flag should override env: got %s", cfg.M3UURL)
    }
}

func TestDefaults(t *testing.T) {
    cfg, err := Load("", nil)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.ListenAddr != ":8080" {
        t.Errorf("default listen_addr wrong")
    }
    if cfg.MaxStreams != 4 {
        t.Errorf("default max_streams wrong")
    }
    if cfg.StreamTimeout != 30*time.Second {
        t.Errorf("default stream_timeout wrong")
    }
    if cfg.FFmpegPath != "ffmpeg" {
        t.Errorf("default ffmpeg_path wrong")
    }
}
```

Run: `go test ./internal/config/... -v`
Expected: FAIL — `Load` undefined.

**Step 2: Implement config structs**

```go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"

    "gopkg.in/yaml.v3"
)

type Config struct {
    M3UURL          string        `yaml:"m3u_url"`
    EPGURL          string        `yaml:"epg_url"`
    ListenAddr      string        `yaml:"listen_addr"`
    MaxStreams      int           `yaml:"max_streams"`
    StreamTimeout   time.Duration `yaml:"stream_timeout"`
    RefreshInterval time.Duration `yaml:"refresh_interval"`
    FFmpegPath      string        `yaml:"ffmpeg_path"`
    DefaultHeaders  Headers       `yaml:"default_headers"`
}

type Headers struct {
    Token     string `yaml:"token"`
    Referer   string `yaml:"referer"`
    UserAgent string `yaml:"user_agent"`
}

func Load(filePath string, flags map[string]string) (*Config, error) {
    cfg := &Config{}
    applyDefaults(cfg)

    if filePath != "" {
        if err := loadFromFile(filePath, cfg); err != nil {
            return nil, err
        }
    }

    applyEnv(cfg)
    applyFlags(cfg, flags)
    return cfg, nil
}

func applyDefaults(cfg *Config) {
    cfg.ListenAddr = ":8080"
    cfg.MaxStreams = 4
    cfg.StreamTimeout = 30 * time.Second
    cfg.RefreshInterval = 1 * time.Hour
    cfg.FFmpegPath = "ffmpeg"
}

func loadFromFile(path string, cfg *Config) error {
    b, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("read config: %w", err)
    }
    return yaml.Unmarshal(b, cfg)
}
```

**Step 3: Implement env loader**

```go
package config

import (
    "os"
    "strconv"
    "strings"
    "time"
)

const envPrefix = "PLEXISHOW_"

func applyEnv(cfg *Config) {
    if v := os.Getenv(envPrefix + "M3U_URL"); v != "" {
        cfg.M3UURL = v
    }
    if v := os.Getenv(envPrefix + "EPG_URL"); v != "" {
        cfg.EPGURL = v
    }
    if v := os.Getenv(envPrefix + "LISTEN_ADDR"); v != "" {
        cfg.ListenAddr = v
    }
    if v := os.Getenv(envPrefix + "MAX_STREAMS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            cfg.MaxStreams = n
        }
    }
    if v := os.Getenv(envPrefix + "STREAM_TIMEOUT"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            cfg.StreamTimeout = d
        }
    }
    if v := os.Getenv(envPrefix + "REFRESH_INTERVAL"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            cfg.RefreshInterval = d
        }
    }
    if v := os.Getenv(envPrefix + "FFMPEG_PATH"); v != "" {
        cfg.FFmpegPath = v
    }
    if v := os.Getenv(envPrefix + "DEFAULT_HEADERS_TOKEN"); v != "" {
        cfg.DefaultHeaders.Token = v
    }
    if v := os.Getenv(envPrefix + "DEFAULT_HEADERS_REFERER"); v != "" {
        cfg.DefaultHeaders.Referer = v
    }
    if v := os.Getenv(envPrefix + "DEFAULT_HEADERS_USER_AGENT"); v != "" {
        cfg.DefaultHeaders.UserAgent = v
    }
}
```

**Step 4: Implement flags loader**

```go
package config

import (
    "strconv"
    "time"
)

func applyFlags(cfg *Config, flags map[string]string) {
    if flags == nil {
        return
    }
    if v, ok := flags["m3u_url"]; ok {
        cfg.M3UURL = v
    }
    if v, ok := flags["epg_url"]; ok {
        cfg.EPGURL = v
    }
    if v, ok := flags["listen_addr"]; ok {
        cfg.ListenAddr = v
    }
    if v, ok := flags["max_streams"]; ok {
        if n, err := strconv.Atoi(v); err == nil {
            cfg.MaxStreams = n
        }
    }
    if v, ok := flags["stream_timeout"]; ok {
        if d, err := time.ParseDuration(v); err == nil {
            cfg.StreamTimeout = d
        }
    }
    if v, ok := flags["refresh_interval"]; ok {
        if d, err := time.ParseDuration(v); err == nil {
            cfg.RefreshInterval = d
        }
    }
    if v, ok := flags["ffmpeg_path"]; ok {
        cfg.FFmpegPath = v
    }
    if v, ok := flags["token"]; ok {
        cfg.DefaultHeaders.Token = v
    }
    if v, ok := flags["referer"]; ok {
        cfg.DefaultHeaders.Referer = v
    }
    if v, ok := flags["user_agent"]; ok {
        cfg.DefaultHeaders.UserAgent = v
    }
}
```

Run: `go test ./internal/config/... -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add unified config loader supporting YAML, env vars, and CLI flags"
```

---

### Task 1.2: Define Domain Channel Model

**Objective:** Create the core `Channel` struct used by all packages.

**Files:**
- Create: `internal/m3u/channel.go`

**Step 1: Write model**

```go
package m3u

// Channel represents a parsed IPTV channel with decryption metadata.
type Channel struct {
    ID       string            `json:"id"`
    Name     string            `json:"name"`
    TVGID    string            `json:"tvg_id,omitempty"`
    TVGLogo  string            `json:"tvg_logo,omitempty"`
    Group    string            `json:"group_title,omitempty"`
    URL      string            `json:"url"`
    KeyID    string            `json:"key_id,omitempty"`    // hex
    Key      string            `json:"key,omitempty"`       // hex
    Headers  map[string]string `json:"headers,omitempty"`   // merged defaults + per-channel
}
```

No test needed for pure struct; tested via parser.

**Step 2: Commit**

```bash
git add internal/m3u/channel.go
git commit -m "feat(m3u): define Channel domain model"
```

---

## Phase 2: M3U Parser (with Fixtures)

### Task 2.1: Download Fixture M3U

**Objective:** Capture the real source M3U as a test fixture.

**Files:**
- Create: `test/fixtures/source.m3u`

**Step 1: Fetch real M3U**

Run:
```bash
curl -sL -o test/fixtures/source.m3u "https://raw.githubusercontent.com/CrisArya/mrstv1/refs/heads/main/mago"
```

Verify file exists and contains `#KODIPROP:inputstream.adaptive.license_key` and `start_time`/`end_time`.

**Step 2: Commit**

```bash
git add test/fixtures/source.m3u
git commit -m "chore: add real M3U fixture"
```

---

### Task 2.2: Implement M3U Parser

**Objective:** Parse EXTINF, KODIPROP, EXTVLCOPT, and URL lines into `[]Channel`.

**Files:**
- Create: `internal/m3u/parser.go`
- Test: `internal/m3u/parser_test.go`

**Step 1: Write failing tests**

```go
package m3u

import (
    "os"
    "testing"
)

func TestParseFixture(t *testing.T) {
    b, err := os.ReadFile("../../test/fixtures/source.m3u")
    if err != nil {
        t.Fatal(err)
    }
    chs, err := Parse(b)
    if err != nil {
        t.Fatalf("parse error: %v", err)
    }
    if len(chs) == 0 {
        t.Fatal("expected channels")
    }
    // Validate first channel has key
    c := chs[0]
    if c.KeyID == "" || c.Key == "" {
        t.Errorf("first channel missing key: %+v", c)
    }
    if c.URL == "" {
        t.Errorf("first channel missing url")
    }
}

func TestParseMinimal(t *testing.T) {
    data := `#EXTM3U
#EXTINF:-1 tvg-id="foo" tvg-logo="http://img" group-title="Sports",Channel A
#KODIPROP:inputstream.adaptive.license_key=abcd1234:efff5678
#EXTVLCOPT:http-referrer=https://tv.movistar.com.pe/
http://example.com/stream.mpd
`
    chs, err := Parse([]byte(data))
    if err != nil {
        t.Fatal(err)
    }
    if len(chs) != 1 {
        t.Fatalf("expected 1 channel, got %d", len(chs))
    }
    c := chs[0]
    if c.Name != "Channel A" {
        t.Errorf("name = %q", c.Name)
    }
    if c.TVGID != "foo" {
        t.Errorf("tvg-id = %q", c.TVGID)
    }
    if c.KeyID != "abcd1234" || c.Key != "efff5678" {
        t.Errorf("key mismatch: %s:%s", c.KeyID, c.Key)
    }
    if c.Headers["Referer"] != "https://tv.movistar.com.pe/" {
        t.Errorf("referer missing")
    }
}
```

Run: `go test ./internal/m3u/... -v`
Expected: FAIL — `Parse` undefined.

**Step 2: Implement parser**

```go
package m3u

import (
    "bufio"
    "bytes"
    "fmt"
    "net/url"
    "regexp"
    "strings"
)

var (
    extinfRe   = regexp.MustCompile(`#EXTINF:-?\d+\s+(.*),(.*)`)
    attrRe     = regexp.MustCompile(`(\S+)="([^"]+)"`)
    kodipropRe = regexp.MustCompile(`#KODIPROP:inputstream\.adaptive\.license_key=([a-fA-F0-9]+):([a-fA-F0-9]+)`)
    vlcoptRe   = regexp.MustCompile(`#EXTVLCOPT:(.+)`)
)

// Parse reads an M3U playlist and returns channels.
func Parse(data []byte) ([]Channel, error) {
    if !bytes.HasPrefix(data, []byte("#EXTM3U")) {
        return nil, fmt.Errorf("missing #EXTM3U header")
    }
    var channels []Channel
    scanner := bufio.NewScanner(bytes.NewReader(data))
    var current *Channel

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#EXTM3U") {
            continue
        }

        if strings.HasPrefix(line, "#EXTINF:") {
            if current != nil {
                channels = append(channels, *current)
            }
            current = &Channel{Headers: make(map[string]string)}
            m := extinfRe.FindStringSubmatch(line)
            if len(m) == 3 {
                attrs := m[1]
                current.Name = strings.TrimSpace(m[2])
                for _, am := range attrRe.FindAllStringSubmatch(attrs, -1) {
                    if len(am) == 3 {
                        switch am[1] {
                        case "tvg-id":
                            current.TVGID = am[2]
                        case "tvg-logo":
                            current.TVGLogo = am[2]
                        case "group-title":
                            current.Group = am[2]
                        }
                    }
                }
            }
            continue
        }

        if strings.HasPrefix(line, "#KODIPROP:") {
            m := kodipropRe.FindStringSubmatch(line)
            if len(m) == 3 && current != nil {
                current.KeyID = strings.ToLower(m[1])
                current.Key = strings.ToLower(m[2])
            }
            continue
        }

        if strings.HasPrefix(line, "#EXTVLCOPT:") {
            m := vlcoptRe.FindStringSubmatch(line)
            if len(m) == 2 && current != nil {
                parseVLCOpt(current, m[1])
            }
            continue
        }

        // URL line
        if current != nil && !strings.HasPrefix(line, "#") {
            current.URL = line
            // Derive ID from tvg-id if present, else URL path or random
            if current.TVGID != "" {
                current.ID = sanitizeID(current.TVGID)
            } else {
                u, _ := url.Parse(line)
                if u != nil {
                    current.ID = sanitizeID(u.Path)
                } else {
                    current.ID = sanitizeID(line)
                }
            }
            channels = append(channels, *current)
            current = nil
        }
    }

    if err := scanner.Err(); err != nil {
        return nil, err
    }
    return channels, nil
}

func parseVLCOpt(ch *Channel, opt string) {
    if strings.HasPrefix(opt, "http-referrer=") {
        ch.Headers["Referer"] = strings.TrimPrefix(opt, "http-referrer=")
    } else if strings.HasPrefix(opt, "http-user-agent=") {
        ch.Headers["User-Agent"] = strings.TrimPrefix(opt, "http-user-agent=")
    } else if strings.HasPrefix(opt, "http-header=") {
        // format: http-header=Key: Value
        rest := strings.TrimPrefix(opt, "http-header=")
        parts := strings.SplitN(rest, ":", 2)
        if len(parts) == 2 {
            ch.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
        }
    }
}

func sanitizeID(s string) string {
    s = strings.ToLower(s)
    var b strings.Builder
    for _, r := range s {
        if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
            b.WriteRune(r)
        } else {
            b.WriteRune('-')
        }
    }
    return strings.Trim(b.String(), "-")
}
```

Run: `go test ./internal/m3u/... -v`
Expected: PASS.

**Step 3: Commit**

```bash
git add internal/m3u/
git commit -m "feat(m3u): implement M3U parser with ClearKey extraction"
```

---

### Task 2.3: Add Parser Edge-Case Tests

**Objective:** Ensure parser handles malformed input gracefully.

**Files:**
- Modify: `internal/m3u/parser_test.go`

**Step 1: Add tests**

```go
func TestParseNoHeader(t *testing.T) {
    _, err := Parse([]byte("http://example.com\n"))
    if err == nil {
        t.Fatal("expected error for missing #EXTM3U")
    }
}

func TestParseDuplicateChannels(t *testing.T) {
    // 155+ fixture already tested; just ensure no panic on duplicate IDs
    b, _ := os.ReadFile("../../test/fixtures/source.m3u")
    chs, _ := Parse(b)
    ids := make(map[string]int)
    for _, c := range chs {
        ids[c.ID]++
    }
    // Duplicates acceptable; proxy will use first or last depending on store policy
}
```

Run: `go test ./internal/m3u/... -v`
Expected: PASS.

**Step 2: Commit**

```bash
git commit -am "test(m3u): add edge-case parser tests"
```

---

## Phase 3: In-Memory Channel Store

### Task 3.1: Implement Thread-Safe Store

**Objective:** Hold channels in memory with safe concurrent access.

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Step 1: Write failing tests**

```go
package store

import (
    "testing"

    "github.com/aymerici/plexishow/internal/m3u"
)

func TestReplaceAndGet(t *testing.T) {
    s := New()
    s.Replace([]m3u.Channel{
        {ID: "foo", Name: "Foo"},
        {ID: "bar", Name: "Bar"},
    })
    c, ok := s.Get("foo")
    if !ok || c.Name != "Foo" {
        t.Errorf("expected Foo")
    }
    all := s.All()
    if len(all) != 2 {
        t.Errorf("expected 2 channels")
    }
}

func TestGetMissing(t *testing.T) {
    s := New()
    _, ok := s.Get("nope")
    if ok {
        t.Error("expected false")
    }
}
```

Run: `go test ./internal/store/... -v`
Expected: FAIL.

**Step 2: Implement store**

```go
package store

import (
    "sync"

    "github.com/aymerici/plexishow/internal/m3u"
)

// Store holds channels in memory.
type Store struct {
    mu   sync.RWMutex
    data map[string]m3u.Channel
    list []m3u.Channel
}

func New() *Store {
    return &Store{data: make(map[string]m3u.Channel)}
}

func (s *Store) Replace(chs []m3u.Channel) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.list = make([]m3u.Channel, len(chs))
    copy(s.list, chs)
    s.data = make(map[string]m3u.Channel, len(chs))
    for _, c := range chs {
        s.data[c.ID] = c
    }
}

func (s *Store) Get(id string) (m3u.Channel, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    c, ok := s.data[id]
    return c, ok
}

func (s *Store) All() []m3u.Channel {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]m3u.Channel, len(s.list))
    copy(out, s.list)
    return out
}
```

Run: `go test ./internal/store/... -v`
Expected: PASS.

**Step 3: Commit**

```bash
git add internal/store/
git commit -m "feat(store): add in-memory thread-safe channel store"
```

---

## Phase 4: Periodic M3U Fetcher

### Task 4.1: Implement Fetcher with Merged Headers

**Objective:** Periodically download M3U, parse it, and update the store.

**Files:**
- Create: `internal/m3u/fetcher.go`
- Test: `internal/m3u/fetcher_test.go` (use httptest)

**Step 1: Write failing tests**

```go
package m3u

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/aymerici/plexishow/internal/config"
    "github.com/aymerici/plexishow/internal/store"
)

func TestFetcherPull(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-TCDN-token") != "abc" {
            t.Errorf("missing token header")
        }
        w.Write([]byte(`#EXTM3U
#EXTINF:-1 tvg-id="x",Test
#KODIPROP:inputstream.adaptive.license_key=a:b
http://s/stream.mpd
`))
    }))
    defer srv.Close()

    st := store.New()
    f := NewFetcher(config.Config{
        M3UURL:         srv.URL,
        DefaultHeaders: config.Headers{Token: "abc", Referer: "ref", UserAgent: "ua"},
        StreamTimeout:  5 * time.Second,
    }, st)

    if err := f.Pull(); err != nil {
        t.Fatal(err)
    }
    chs := st.All()
    if len(chs) != 1 {
        t.Fatalf("expected 1 channel, got %d", len(chs))
    }
    if chs[0].Headers["X-TCDN-token"] != "abc" {
        t.Errorf("token not merged")
    }
    if chs[0].Headers["User-Agent"] != "ua" {
        t.Errorf("ua not merged")
    }
}
```

Run: `go test ./internal/m3u/... -run TestFetcherPull -v`
Expected: FAIL — `NewFetcher` undefined.

**Step 2: Implement fetcher**

```go
package m3u

import (
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/aymerici/plexishow/internal/config"
    "github.com/aymerici/plexishow/internal/store"
)

// Fetcher downloads and parses the M3U periodically.
type Fetcher struct {
    cfg   config.Config
    store *store.Store
    client *http.Client
}

func NewFetcher(cfg config.Config, s *store.Store) *Fetcher {
    return &Fetcher{
        cfg: cfg,
        store: s,
        client: &http.Client{Timeout: cfg.StreamTimeout},
    }
}

func (f *Fetcher) Pull() error {
    req, err := http.NewRequest("GET", f.cfg.M3UURL, nil)
    if err != nil {
        return err
    }
    if f.cfg.DefaultHeaders.Token != "" {
        req.Header.Set("X-TCDN-token", f.cfg.DefaultHeaders.Token)
    }
    if f.cfg.DefaultHeaders.Referer != "" {
        req.Header.Set("Referer", f.cfg.DefaultHeaders.Referer)
    }
    if f.cfg.DefaultHeaders.UserAgent != "" {
        req.Header.Set("User-Agent", f.cfg.DefaultHeaders.UserAgent)
    }

    resp, err := f.client.Do(req)
    if err != nil {
        return fmt.Errorf("fetch m3u: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("fetch m3u: status %d", resp.StatusCode)
    }

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("read m3u: %w", err)
    }
    channels, err := Parse(data)
    if err != nil {
        return fmt.Errorf("parse m3u: %w", err)
    }

    // Merge default headers into each channel
    for i := range channels {
        if channels[i].Headers == nil {
            channels[i].Headers = make(map[string]string)
        }
        if _, ok := channels[i].Headers["X-TCDN-token"]; !ok && f.cfg.DefaultHeaders.Token != "" {
            channels[i].Headers["X-TCDN-token"] = f.cfg.DefaultHeaders.Token
        }
        if _, ok := channels[i].Headers["Referer"]; !ok && f.cfg.DefaultHeaders.Referer != "" {
            channels[i].Headers["Referer"] = f.cfg.DefaultHeaders.Referer
        }
        if _, ok := channels[i].Headers["User-Agent"]; !ok && f.cfg.DefaultHeaders.UserAgent != "" {
            channels[i].Headers["User-Agent"] = f.cfg.DefaultHeaders.UserAgent
        }
    }

    f.store.Replace(channels)
    return nil
}

func (f *Fetcher) Start() {
    go func() {
        ticker := time.NewTicker(f.cfg.RefreshInterval)
        defer ticker.Stop()
        if err := f.Pull(); err != nil {
            // TODO: logging
        }
        for range ticker.C {
            if err := f.Pull(); err != nil {
                // TODO: logging
            }
        }
    }()
}
```

Run: `go test ./internal/m3u/... -run TestFetcherPull -v`
Expected: PASS.

**Step 3: Commit**

```bash
git add internal/m3u/fetcher.go internal/m3u/fetcher_test.go
git commit -m "feat(m3u): add periodic fetcher with header merging"
```

---

## Phase 5: HDHomeRun API Surface

### Task 5.1: Define HDHomeRun Structs

**Objective:** Model the JSON/XML responses Plex expects.

**Files:**
- Create: `internal/hdhr/types.go`

**Step 1: Write structs**

```go
package hdhr

// DeviceDescription matches /device.xml UPnP response.
type DeviceDescription struct {
    XMLName        string `xml:"root"`
    SpecMajor      int    `xml:"specVersion>major"`
    SpecMinor      int    `xml:"specVersion>minor"`
    URLBase        string `xml:"URLBase"`
    DeviceType     string `xml:"device>deviceType"`
    FriendlyName   string `xml:"device>friendlyName"`
    Manufacturer   string `xml:"device>manufacturer"`
    ModelNumber    string `xml:"device>modelNumber"`
    ModelName      string `xml:"device>modelName"`
    SerialNumber   string `xml:"device>serialNumber"`
}

// Discover matches /discover.json.
type Discover struct {
    FriendlyName    string `json:"FriendlyName"`
    Manufacturer    string `json:"Manufacturer"`
    ModelNumber     string `json:"ModelNumber"`
    FirmwareName    string `json:"FirmwareName"`
    TunerCount      int    `json:"TunerCount"`
    FirmwareVersion string `json:"FirmwareVersion"`
    DeviceID        string `json:"DeviceID"`
    DeviceAuth      string `json:"DeviceAuth"`
    BaseURL         string `json:"BaseURL"`
    LineupURL       string `json:"LineupURL"`
}

// LineupStatus matches /lineup_status.json.
type LineupStatus struct {
    ScanInProgress int    `json:"ScanInProgress"`
    ScanPossible   int    `json:"ScanPossible"`
    Source         string `json:"Source"`
    SourceList     []string `json:"SourceList"`
}

// LineupItem matches /lineup.json entries.
type LineupItem struct {
    GuideNumber string `json:"GuideNumber"`
    GuideName   string `json:"GuideName"`
    URL         string `json:"URL"`
    HD          int    `json:"HD,omitempty"`
    HDHRURL     string `json:"HDHomeRunURL,omitempty"`
}
```

**Step 2: Commit**

```bash
git add internal/hdhr/types.go
git commit -m "feat(hdhr): define HDHomeRun API types"
```

---

### Task 5.2: Implement HDHomeRun Handlers

**Objective:** Wire `/device.xml`, `/discover.json`, `/lineup.json`, `/lineup_status.json`.

**Files:**
- Create: `internal/hdhr/handler.go`
- Test: `internal/hdhr/handler_test.go`

**Step 1: Write failing tests**

```go
package hdhr

import (
    "encoding/json"
    "encoding/xml"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/aymerici/plexishow/internal/m3u"
    "github.com/aymerici/plexishow/internal/store"
)

func TestDiscover(t *testing.T) {
    st := store.New()
    h := NewHandler("http://localhost:8080", st)
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/discover.json", nil)
    h.Discover(w, r)
    if w.Code != 200 {
        t.Fatalf("status %d", w.Code)
    }
    var d Discover
    if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
        t.Fatal(err)
    }
    if d.TunerCount != 4 {
        t.Errorf("tunercount = %d", d.TunerCount)
    }
}

func TestLineup(t *testing.T) {
    st := store.New()
    st.Replace([]m3u.Channel{
        {ID: "ch1", Name: "One", TVGID: "1.1"},
    })
    h := NewHandler("http://localhost:8080", st)
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/lineup.json", nil)
    h.Lineup(w, r)
    var items []LineupItem
    if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
        t.Fatal(err)
    }
    if len(items) != 1 || items[0].GuideName != "One" {
        t.Errorf("unexpected lineup: %+v", items)
    }
}

func TestDeviceXML(t *testing.T) {
    st := store.New()
    h := NewHandler("http://localhost:8080", st)
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/device.xml", nil)
    h.DeviceXML(w, r)
    var d DeviceDescription
    if err := xml.Unmarshal(w.Body.Bytes(), &d); err != nil {
        t.Fatal(err)
    }
    if d.FriendlyName == "" {
        t.Error("empty friendly name")
    }
}
```

Run: `go test ./internal/hdhr/... -v`
Expected: FAIL.

**Step 2: Implement handler**

```go
package hdhr

import (
    "encoding/json"
    "encoding/xml"
    "fmt"
    "net/http"

    "github.com/aymerici/plexishow/internal/store"
)

// Handler serves HDHomeRun-compatible endpoints.
type Handler struct {
    baseURL string
    store   *store.Store
}

func NewHandler(baseURL string, s *store.Store) *Handler {
    return &Handler{baseURL: baseURL, store: s}
}

func (h *Handler) DeviceXML(w http.ResponseWriter, r *http.Request) {
    d := DeviceDescription{
        SpecMajor:    1,
        SpecMinor:    0,
        URLBase:      h.baseURL,
        DeviceType:   "urn:schemas-upnp-org:device:MediaServer:1",
        FriendlyName: "Plexishow",
        Manufacturer: "Silicondust",
        ModelNumber:  "HDHR3-US",
        ModelName:    "HDHomeRun",
        SerialNumber: "12345678",
    }
    w.Header().Set("Content-Type", "application/xml")
    w.Write([]byte(xml.Header))
    enc := xml.NewEncoder(w)
    enc.Indent("", "  ")
    _ = enc.Encode(d)
}

func (h *Handler) Discover(w http.ResponseWriter, r *http.Request) {
    d := Discover{
        FriendlyName:    "Plexishow",
        Manufacturer:    "Silicondust",
        ModelNumber:     "HDHR3-US",
        FirmwareName:    "hdhomerun3_atsc",
        TunerCount:      4,
        FirmwareVersion: "20240101",
        DeviceID:        "12345678",
        DeviceAuth:      "testauth",
        BaseURL:         h.baseURL,
        LineupURL:       h.baseURL + "/lineup.json",
    }
    writeJSON(w, d)
}

func (h *Handler) LineupStatus(w http.ResponseWriter, r *http.Request) {
    s := LineupStatus{
        ScanInProgress: 0,
        ScanPossible:   1,
        Source:         "Cable",
        SourceList:     []string{"Antenna", "Cable"},
    }
    writeJSON(w, s)
}

func (h *Handler) Lineup(w http.ResponseWriter, r *http.Request) {
    channels := h.store.All()
    items := make([]LineupItem, len(channels))
    for i, ch := range channels {
        gn := ch.TVGID
        if gn == "" {
            gn = ch.ID
        }
        items[i] = LineupItem{
            GuideNumber: gn,
            GuideName:   ch.Name,
            URL:         fmt.Sprintf("%s/stream/%s", h.baseURL, ch.ID),
            HD:          1,
        }
    }
    writeJSON(w, items)
}

func writeJSON(w http.ResponseWriter, v any) {
    w.Header().Set("Content-Type", "application/json")
    b, _ := json.MarshalIndent(v, "", "  ")
    w.Write(b)
}
```

Run: `go test ./internal/hdhr/... -v`
Expected: PASS.

**Step 3: Commit**

```bash
git add internal/hdhr/
git commit -m "feat(hdhr): implement HDHomeRun API handlers"
```

---

## Phase 6: Clean M3U & XMLTV EPG Endpoints

### Task 6.1: Clean M3U Endpoint

**Objective:** Serve `/channels.m3u` with proxy URLs and no KODIPROP.

**Files:**
- Create: `internal/server/channels.go`
- Test: `internal/server/channels_test.go`

**Step 1: Write failing test**

```go
package server

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/aymerici/plexishow/internal/m3u"
    "github.com/aymerici/plexishow/internal/store"
)

func TestServeM3U(t *testing.T) {
    st := store.New()
    st.Replace([]m3u.Channel{
        {ID: "c1", Name: "Ch1", TVGID: "1.1", TVGLogo: "http://logo", Group: "Sports", URL: "http://src"},
    })
    srv := New("http://localhost:8080", st, nil, nil)
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/channels.m3u", nil)
    srv.ServeM3U(w, r)
    body := w.Body.String()
    if !strings.Contains(body, "#EXTM3U") {
        t.Error("missing header")
    }
    if !strings.Contains(body, "http://localhost:8080/stream/c1") {
        t.Error("missing proxy url")
    }
    if strings.Contains(body, "KODIPROP") {
        t.Error("should not contain KODIPROP")
    }
}
```

Run: `go test ./internal/server/... -run TestServeM3U -v`
Expected: FAIL — `New` undefined.

**Step 2: Implement endpoint**

```go
package server

import (
    "fmt"
    "net/http"
    "strings"

    "github.com/aymerici/plexishow/internal/store"
)

// Server holds HTTP handlers.
type Server struct {
    baseURL string
    store   *store.Store
    // streamer and epg wired later
    streamer Streamer
    epg      EPGSource
}

type Streamer interface{}
type EPGSource interface{}

func New(baseURL string, st *store.Store, streamer Streamer, epg EPGSource) *Server {
    return &Server{baseURL: baseURL, store: st, streamer: streamer, epg: epg}
}

func (s *Server) ServeM3U(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
    var b strings.Builder
    b.WriteString("#EXTM3U\n")
    for _, ch := range s.store.All() {
        fmt.Fprintf(&b, "#EXTINF:-1 tvg-id=%q tvg-logo=%q group-title=%q,%s\n",
            ch.TVGID, ch.TVGLogo, ch.Group, ch.Name)
        fmt.Fprintf(&b, "%s/stream/%s\n", s.baseURL, ch.ID)
    }
    w.Write([]byte(b.String()))
}
```

Run: `go test ./internal/server/... -run TestServeM3U -v`
Expected: PASS.

**Step 3: Commit**

```bash
git add internal/server/channels.go internal/server/channels_test.go
git commit -m "feat(server): add clean M3U endpoint"
```

---

### Task 6.2: XMLTV EPG Passthrough/Transform

**Objective:** Serve `/epg.xml` from configured `epg_url`, rewriting channel IDs if needed.

**Files:**
- Create: `internal/epg/epg.go`
- Test: `internal/epg/epg_test.go` (use httptest for source)

**Step 1: Write failing test**

```go
package epg

import (
    "io"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestFetchAndServe(t *testing.T) {
    src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/xml")
        w.Write([]byte(`<?xml version="1.0"?><tv><channel id="foo"></channel></tv>`))
    }))
    defer src.Close()

    e := New(src.URL, &http.Client{})
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/epg.xml", nil)
    e.ServeHTTP(w, r)
    body, _ := io.ReadAll(w.Result().Body)
    if !strings.Contains(string(body), `<channel id="foo">`) {
        t.Errorf("unexpected body: %s", body)
    }
}
```

Run: `go test ./internal/epg/... -v`
Expected: FAIL.

**Step 2: Implement EPG source**

```go
package epg

import (
    "io"
    "net/http"
    "time"
)

// Source fetches XMLTV from a remote URL and serves it.
type Source struct {
    url    string
    client *http.Client
    cache  []byte
    etag   string
    mu     sync.RWMutex
    last   time.Time
}

func New(url string, client *http.Client) *Source {
    if client == nil {
        client = &http.Client{Timeout: 30 * time.Second}
    }
    return &Source{url: url, client: client}
}

func (s *Source) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.mu.RLock()
    data := s.cache
    s.mu.RUnlock()
    if len(data) == 0 {
        http.Error(w, "EPG not available", http.StatusServiceUnavailable)
        return
    }
    w.Header().Set("Content-Type", "application/xml")
    w.Write(data)
}

func (s *Source) Refresh() error {
    req, err := http.NewRequest("GET", s.url, nil)
    if err != nil {
        return err
    }
    resp, err := s.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("epg fetch status %d", resp.StatusCode)
    }
    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    s.mu.Lock()
    s.cache = data
    s.last = time.Now()
    s.mu.Unlock()
    return nil
}
```

Run: `go test ./internal/epg/... -v`
Expected: PASS.

**Step 3: Commit**

```bash
git add internal/epg/
git commit -m "feat(epg): add XMLTV passthrough with caching"
```

---

## Phase 7: FFmpeg Streaming Engine

### Task 7.1: Define Streamer Interface and Process Model

**Objective:** On `/stream/{id}`, spawn ffmpeg with per-channel ClearKey and headers, pipe stdout to HTTP response, manage lifecycle.

**Files:**
- Create: `internal/stream/streamer.go`
- Test: `internal/stream/streamer_test.go`

**Step 1: Write failing test**

```go
package stream

import (
    "context"
    "io"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/aymerici/plexishow/internal/config"
    "github.com/aymerici/plexishow/internal/m3u"
    "github.com/aymerici/plexishow/internal/store"
)

func TestServeChannel(t *testing.T) {
    st := store.New()
    st.Replace([]m3u.Channel{
        {ID: "c1", Name: "Ch1", URL: "http://example.com/stream.mpd", KeyID: "a", Key: "b"},
    })
    cfg := config.Config{MaxStreams: 2, StreamTimeout: 5 * time.Second, FFmpegPath: "ffmpeg"}
    sm := NewManager(cfg, st)

    // Use a fake ffmpeg that outputs something harmless
    // We can't easily mock exec.Command here without interface; design for interface.
    // Instead test that ServeHTTP rejects unknown channel.
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/stream/c2", nil)
    sm.ServeHTTP(w, r)
    if w.Code != http.StatusNotFound {
        t.Errorf("expected 404, got %d", w.Code)
    }
}
```

Run: `go test ./internal/stream/... -v`
Expected: FAIL.

**Step 2: Implement streamer with concurrency limit**

```go
package stream

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "os/exec"
    "strings"
    "sync"
    "time"

    "github.com/aymerici/plexishow/internal/config"
    "github.com/aymerici/plexishow/internal/store"
)

// Manager handles ffmpeg lifecycle per stream.
type Manager struct {
    cfg      config.Config
    store    *store.Store
    mu       sync.Mutex
    active   map[string]*exec.Cmd
    sem      chan struct{}
}

func NewManager(cfg config.Config, st *store.Store) *Manager {
    return &Manager{
        cfg:    cfg,
        store:  st,
        active: make(map[string]*exec.Cmd),
        sem:    make(chan struct{}, cfg.MaxStreams),
    }
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/stream/")
    ch, ok := m.store.Get(id)
    if !ok {
        http.Error(w, "channel not found", http.StatusNotFound)
        return
    }

    select {
    case m.sem <- struct{}{}:
    default:
        http.Error(w, "max concurrent streams reached", http.StatusServiceUnavailable)
        return
    }
    defer func() { <-m.sem }()

    ctx, cancel := context.WithCancel(r.Context())
    defer cancel()

    args := buildArgs(m.cfg, ch)
    cmd := exec.CommandContext(ctx, m.cfg.FFmpegPath, args...)

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    stderr, _ := cmd.StderrPipe()
    defer stdout.Close()
    if stderr != nil {
        go io.Copy(io.Discard, stderr) // or log
    }

    if err := cmd.Start(); err != nil {
        http.Error(w, "failed to start stream", http.StatusInternalServerError)
        return
    }
    m.track(ch.ID, cmd)
    defer m.untrack(ch.ID)
    defer func() {
        if cmd.Process != nil {
            _ = cmd.Process.Kill()
        }
        _ = cmd.Wait()
    }()

    w.Header().Set("Content-Type", "video/mp2t")
    w.Header().Set("Transfer-Encoding", "chunked")
    w.WriteHeader(http.StatusOK)

    // Flush headers
    if f, ok := w.(http.Flusher); ok {
        f.Flush()
    }

    // Copy with idle timeout
    timer := time.NewTimer(m.cfg.StreamTimeout)
    defer timer.Stop()
    done := make(chan struct{})
    go func() {
        _, _ = io.Copy(w, stdout)
        close(done)
    }()

    select {
    case <-done:
    case <-ctx.Done():
    case <-timer.C:
    }
}

func (m *Manager) track(id string, cmd *exec.Cmd) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.active[id] = cmd
}

func (m *Manager) untrack(id string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.active, id)
}

func (m *Manager) Shutdown() {
    m.mu.Lock()
    defer m.mu.Unlock()
    for _, cmd := range m.active {
        if cmd.Process != nil {
            _ = cmd.Process.Kill()
        }
    }
}

func buildArgs(cfg config.Config, ch m3u.Channel) []string {
    args := []string{
        "-fflags", "+discardcorrupt",
        "-headers", buildHeaders(ch.Headers),
        "-i", ch.URL,
        "-c:v", "copy",
        "-c:a", "aac",
        "-f", "mpegts",
        "-",
    }
    if ch.KeyID != "" && ch.Key != "" {
        // Insert cenc decryption key before input
        newArgs := make([]string, 0, len(args)+4)
        newArgs = append(newArgs, args[0:2]...) // -fflags +discardcorrupt
        newArgs = append(newArgs, "-cenc_decryption_key", fmt.Sprintf("%s:%s", ch.KeyID, ch.Key))
        newArgs = append(newArgs, args[2:]...)
        args = newArgs
    }
    return args
}

func buildHeaders(h map[string]string) string {
    var b strings.Builder
    for k, v := range h {
        fmt.Fprintf(&b, "%s: %s\r\n", k, v)
    }
    return b.String()
}
```

Run: `go test ./internal/stream/... -v`
Expected: PASS (only 404 test so far).

**Step 3: Commit**

```bash
git add internal/stream/
git commit -m "feat(stream): add ffmpeg streaming manager with concurrency limit"
```

---

### Task 7.2: Streamer Lifecycle Tests

**Objective:** Test concurrency limit and shutdown.

**Files:**
- Modify: `internal/stream/streamer_test.go`

**Step 1: Add tests**

```go
func TestConcurrencyLimit(t *testing.T) {
    st := store.New()
    st.Replace([]m3u.Channel{
        {ID: "c1", Name: "Ch1", URL: "http://example.com/a.mpd", KeyID: "a", Key: "b"},
        {ID: "c2", Name: "Ch2", URL: "http://example.com/b.mpd", KeyID: "c", Key: "d"},
    })
    cfg := config.Config{MaxStreams: 1, StreamTimeout: 1 * time.Second, FFmpegPath: "ffmpeg"}
    sm := NewManager(cfg, st)

    // Simulate holding the single slot
    sm.sem <- struct{}{}
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/stream/c2", nil)
    sm.ServeHTTP(w, r)
    if w.Code != http.StatusServiceUnavailable {
        t.Errorf("expected 503, got %d", w.Code)
    }
    <-sm.sem
}

func TestShutdown(t *testing.T) {
    st := store.New()
    cfg := config.Config{MaxStreams: 2, StreamTimeout: 1 * time.Second, FFmpegPath: "ffmpeg"}
    sm := NewManager(cfg, st)
    // Should not panic even if empty
    sm.Shutdown()
}
```

Run: `go test ./internal/stream/... -v`
Expected: PASS.

**Step 3: Commit**

```bash
git commit -am "test(stream): add concurrency and shutdown tests"
```

---

## Phase 8: HTTP Server & Graceful Shutdown

### Task 8.1: Wire All Routes in Main Server

**Objective:** Compose hdhr, channels, epg, and stream handlers into one HTTP server.

**Files:**
- Create: `internal/server/server.go`
- Modify: `internal/server/channels.go` (remove Streamer/EPG placeholders, use real deps)

**Step 1: Rewrite server.go**

```go
package server

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/aymerici/plexishow/internal/epg"
    "github.com/aymerici/plexishow/internal/hdhr"
    "github.com/aymerici/plexishow/internal/store"
    "github.com/aymerici/plexishow/internal/stream"
)

// Server holds HTTP handlers.
type Server struct {
    baseURL  string
    store    *store.Store
    streamer *stream.Manager
    epg      *epg.Source
    hdhr     *hdhr.Handler
}

func New(baseURL string, st *store.Store, streamer *stream.Manager, epg *epg.Source) *Server {
    return &Server{
        baseURL:  baseURL,
        store:    st,
        streamer: streamer,
        epg:      epg,
        hdhr:     hdhr.NewHandler(baseURL, st),
    }
}

func (s *Server) Router() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/device.xml", s.hdhr.DeviceXML)
    mux.HandleFunc("/discover.json", s.hdhr.Discover)
    mux.HandleFunc("/lineup.json", s.hdhr.Lineup)
    mux.HandleFunc("/lineup_status.json", s.hdhr.LineupStatus)
    mux.HandleFunc("/channels.m3u", s.ServeM3U)
    if s.epg != nil {
        mux.HandleFunc("/epg.xml", s.epg.ServeHTTP)
    }
    mux.HandleFunc("/stream/", s.streamer.ServeHTTP)
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    })
    return mux
}

func (s *Server) Run(ctx context.Context, addr string) error {
    srv := &http.Server{
        Addr:    addr,
        Handler: s.Router(),
    }
    go func() {
        <-ctx.Done()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        _ = srv.Shutdown(shutdownCtx)
    }()
    fmt.Printf("Listening on %s\n", addr)
    return srv.ListenAndServe()
}
```

Update `channels.go` to remove the `Streamer`/`EPGSource` interfaces and the `New` placeholder from earlier; keep only `ServeM3U`.

**Step 2: Commit**

```bash
git add internal/server/
git commit -m "feat(server): wire all routes and graceful shutdown"
```

---

### Task 8.2: Create Main Entrypoint

**Objective:** Assemble all components in `cmd/plexishow/main.go`.

**Files:**
- Create: `cmd/plexishow/main.go`

**Step 1: Write main**

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "github.com/aymerici/plexishow/internal/config"
    "github.com/aymerici/plexishow/internal/epg"
    "github.com/aymerici/plexishow/internal/m3u"
    "github.com/aymerici/plexishow/internal/server"
    "github.com/aymerici/plexishow/internal/store"
    "github.com/aymerici/plexishow/internal/stream"
)

func main() {
    configPath := flag.String("config", "config.yaml", "Path to config file")
    m3uURL := flag.String("m3u-url", "", "M3U playlist URL (overrides config/env)")
    epgURL := flag.String("epg-url", "", "EPG XMLTV URL (overrides config/env)")
    listenAddr := flag.String("listen-addr", "", "HTTP listen address (overrides config/env)")
    maxStreams := flag.Int("max-streams", 0, "Max concurrent streams (overrides config/env)")
    streamTimeout := flag.String("stream-timeout", "", "Per-stream idle timeout (overrides config/env)")
    refreshInterval := flag.String("refresh-interval", "", "M3U refresh interval (overrides config/env)")
    ffmpegPath := flag.String("ffmpeg-path", "", "Path to ffmpeg binary (overrides config/env)")
    token := flag.String("token", "", "X-TCDN-token header value (overrides config/env)")
    referer := flag.String("referer", "", "Referer header value (overrides config/env)")
    userAgent := flag.String("user-agent", "", "User-Agent header value (overrides config/env)")
    flag.Parse()

    flags := make(map[string]string)
    if *m3uURL != "" { flags["m3u_url"] = *m3uURL }
    if *epgURL != "" { flags["epg_url"] = *epgURL }
    if *listenAddr != "" { flags["listen_addr"] = *listenAddr }
    if *maxStreams > 0 { flags["max_streams"] = strconv.Itoa(*maxStreams) }
    if *streamTimeout != "" { flags["stream_timeout"] = *streamTimeout }
    if *refreshInterval != "" { flags["refresh_interval"] = *refreshInterval }
    if *ffmpegPath != "" { flags["ffmpeg_path"] = *ffmpegPath }
    if *token != "" { flags["token"] = *token }
    if *referer != "" { flags["referer"] = *referer }
    if *userAgent != "" { flags["user_agent"] = *userAgent }

    cfg, err := config.Load(*configPath, flags)
    if err != nil {
        fmt.Fprintf(os.Stderr, "load config: %v\n", err)
        os.Exit(1)
    }

    st := store.New()

    fetcher := m3u.NewFetcher(*cfg, st)
    if err := fetcher.Pull(); err != nil {
        fmt.Fprintf(os.Stderr, "initial m3u fetch: %v\n", err)
        os.Exit(1)
    }
    fetcher.Start()

    var epgSource *epg.Source
    if cfg.EPGURL != "" {
        epgSource = epg.New(cfg.EPGURL, &http.Client{Timeout: 30 * time.Second})
        if err := epgSource.Refresh(); err != nil {
            fmt.Fprintf(os.Stderr, "initial epg fetch: %v\n", err)
        }
        go func() {
            ticker := time.NewTicker(cfg.RefreshInterval)
            defer ticker.Stop()
            for range ticker.C {
                _ = epgSource.Refresh()
            }
        }()
    }

    streamer := stream.NewManager(*cfg, st)

    baseURL := fmt.Sprintf("http://%s", cfg.ListenAddr)
    srv := server.New(baseURL, st, streamer, epgSource)

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    if err := srv.Run(ctx, cfg.ListenAddr); err != nil && err != http.ErrServerClosed {
        fmt.Fprintf(os.Stderr, "server: %v\n", err)
        os.Exit(1)
    }

    streamer.Shutdown()
    fmt.Println("shutdown complete")
}
```

**Step 2: Commit**

```bash
git add cmd/plexishow/main.go
git commit -m "feat(main): add entrypoint wiring all services"
```

---

## Phase 9: Observability (Health & Metrics)

### Task 9.1: Add Prometheus Metrics

**Objective:** Export active streams and errors.

**Files:**
- Create: `internal/metrics/metrics.go`
- Modify: `internal/stream/streamer.go` (instrument active streams)

**Step 1: Write metrics package**

```go
package metrics

import (
    "net/http"
    "strconv"
    "sync/atomic"
)

// Registry holds simple counters.
type Registry struct {
    activeStreams int64
    streamErrors  int64
}

func New() *Registry {
    return &Registry{}
}

func (r *Registry) IncActive() { atomic.AddInt64(&r.activeStreams, 1) }
func (r *Registry) DecActive() { atomic.AddInt64(&r.activeStreams, -1) }
func (r *Registry) IncErrors() { atomic.AddInt64(&r.streamErrors, 1) }

func (r *Registry) Handler() http.HandlerFunc {
    return func(w http.ResponseWriter, req *http.Request) {
        w.Header().Set("Content-Type", "text/plain; version=0.0.4")
        fmt.Fprintf(w, "# HELP plexishow_active_streams Current active ffmpeg streams\n")
        fmt.Fprintf(w, "# TYPE plexishow_active_streams gauge\n")
        fmt.Fprintf(w, "plexishow_active_streams %d\n", atomic.LoadInt64(&r.activeStreams))
        fmt.Fprintf(w, "# HELP plexishow_stream_errors_total Total stream errors\n")
        fmt.Fprintf(w, "# TYPE plexishow_stream_errors_total counter\n")
        fmt.Fprintf(w, "plexishow_stream_errors_total %d\n", atomic.LoadInt64(&r.streamErrors))
    }
}
```

**Step 2: Instrument streamer**

In `internal/stream/streamer.go`, add a `metrics *metrics.Registry` field to `Manager` and call `IncActive/DecActive/IncErrors` around the copy loop and error paths. Update `NewManager` signature to accept it.

**Step 3: Add `/metrics` route**

In `internal/server/server.go`, add:
```go
mux.HandleFunc("/metrics", s.metrics.Handler())
```

**Step 4: Commit**

```bash
git add internal/metrics/ internal/stream/streamer.go internal/server/server.go
git commit -m "feat(metrics): add prometheus-style active stream and error counters"
```

---

## Phase 10: Dockerfile & Multi-Arch Build

### Task 10.1: Create Dockerfile

**Objective:** Multi-stage build with ffmpeg baked in, targeting amd64 and arm64.

**Files:**
- Create: `Dockerfile`

**Step 1: Write Dockerfile**

```dockerfile
# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /bin/plexishow ./cmd/plexishow

FROM alpine:3.19
RUN apk add --no-cache ffmpeg ca-certificates
COPY --from=builder /bin/plexishow /usr/local/bin/plexishow
EXPOSE 8080
ENTRYPOINT ["plexishow"]
CMD ["-config", "/etc/plexishow/config.yaml"]
```

**Step 2: Build locally**

Run:
```bash
docker build -t plexishow:local .
```

Expected: Image builds successfully.

**Step 3: Commit**

```bash
git add Dockerfile
git commit -m "build: add multi-stage Dockerfile with ffmpeg"
```

---

### Task 10.2: Add Build Script for Multi-Arch

**Objective:** Script or Makefile target for `linux/amd64` and `linux/arm64`.

**Files:**
- Create: `Makefile`

**Step 1: Write Makefile**

```makefile
BINARY := plexishow
IMAGE  := plexishow

.PHONY: build test docker-multi

build:
	go build -o bin/$(BINARY) ./cmd/plexishow

test:
	go test ./...

docker-multi:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE):latest --push .
```

**Step 2: Commit**

```bash
git add Makefile
git commit -m "build: add Makefile with multi-arch docker target"
```

---

## Phase 11: Integration & Acceptance

### Task 11.1: Add Example Config

**Objective:** Provide a reference `config.yaml`.

**Files:**
- Create: `config.example.yaml`

**Step 1: Write example**

```yaml
m3u_url: "https://raw.githubusercontent.com/CrisArya/mrstv1/refs/heads/main/mago"
# epg_url: "https://example.com/epg.xml"
listen_addr: ":8080"
max_streams: 4
stream_timeout: "30s"
refresh_interval: "1h"
ffmpeg_path: "ffmpeg"
default_headers:
  token: "Bearer YOUR_TOKEN_HERE"
  referer: "https://tv.movistar.com.pe/"
  user_agent: "Mozilla/5.0 (X11; Linux x86_64)"
```

**Step 2: Commit**

```bash
git add config.example.yaml
git commit -m "docs: add example configuration"
```

---

### Task 11.2: Run Full Test Suite

**Objective:** Ensure all packages pass.

Run:
```bash
go test ./... -v
```

Expected: All tests pass.

If failures, fix and commit individually.

---

### Task 11.3: Static Analysis & Formatting

**Objective:** Enforce Go standards.

Run:
```bash
go fmt ./...
go vet ./...
```

Fix any issues, then commit.

---

### Task 11.4: Integration Smoke Test (Manual)

**Objective:** Verify the binary starts and responds.

Run:
```bash
cp config.example.yaml /tmp/config.yaml
# edit /tmp/config.yaml with a real token
go run ./cmd/plexishow -config /tmp/config.yaml &
PID=$!
sleep 2
curl -s http://localhost:8080/discover.json | jq .
curl -s http://localhost:8080/lineup.json | jq .
curl -s http://localhost:8080/channels.m3u | head -n 10
curl -s http://localhost:8080/health
kill $PID
```

Expected: JSON/M3U responses are valid and populated.

**Commit:**
```bash
git commit -am "docs: verify integration smoke test"
```

---

## CLI & Environment Variables Reference

### Configuration Precedence

```
CLI flags > Environment variables > Config file (config.yaml) > Hardcoded defaults
```

### CLI Flags

| Flag | Description | Example |
|------|-------------|---------|
| `-config` | Path to config file | `-config /etc/plexishow/config.yaml` |
| `-m3u-url` | M3U playlist URL | `-m3u-url https://example.com/playlist.m3u` |
| `-epg-url` | EPG XMLTV URL | `-epg-url https://example.com/epg.xml` |
| `-listen-addr` | HTTP listen address | `-listen-addr :8080` |
| `-max-streams` | Max concurrent streams | `-max-streams 4` |
| `-stream-timeout` | Per-stream idle timeout | `-stream-timeout 30s` |
| `-refresh-interval` | M3U refresh interval | `-refresh-interval 1h` |
| `-ffmpeg-path` | Path to ffmpeg binary | `-ffmpeg-path /usr/bin/ffmpeg` |
| `-token` | X-TCDN-token header | `-token "Bearer abc"` |
| `-referer` | Referer header | `-referer https://tv.movistar.com.pe/` |
| `-user-agent` | User-Agent header | `-user-agent "Mozilla/5.0"` |

### Environment Variables

All env vars use the `PLEXISHOW_` prefix:

| Env Var | Maps To |
|---------|---------|
| `PLEXISHOW_M3U_URL` | `m3u_url` |
| `PLEXISHOW_EPG_URL` | `epg_url` |
| `PLEXISHOW_LISTEN_ADDR` | `listen_addr` |
| `PLEXISHOW_MAX_STREAMS` | `max_streams` |
| `PLEXISHOW_STREAM_TIMEOUT` | `stream_timeout` |
| `PLEXISHOW_REFRESH_INTERVAL` | `refresh_interval` |
| `PLEXISHOW_FFMPEG_PATH` | `ffmpeg_path` |
| `PLEXISHOW_DEFAULT_HEADERS_TOKEN` | `default_headers.token` |
| `PLEXISHOW_DEFAULT_HEADERS_REFERER` | `default_headers.referer` |
| `PLEXISHOW_DEFAULT_HEADERS_USER_AGENT` | `default_headers.user_agent` |

### Usage Examples

**Config file only:**
```bash
./plexishow -config config.yaml
```

**Environment variables:**
```bash
export PLEXISHOW_M3U_URL="https://example.com/playlist.m3u"
export PLEXISHOW_TOKEN="Bearer secret"
./plexishow
```

**CLI flags (highest precedence):**
```bash
./plexishow -m3u-url https://example.com/playlist.m3u -token "Bearer secret" -max-streams 8
```

**Docker with env vars:**
```bash
docker run -e PLEXISHOW_M3U_URL=https://example.com/pl.m3u -e PLEXISHOW_TOKEN=abc plexishow
```

---

## Summary of Key Files

| File | Purpose |
|------|---------|
| `cmd/plexishow/main.go` | Entrypoint |
| `internal/config/config.go` | YAML loader |
| `internal/m3u/parser.go` | M3U + ClearKey parser |
| `internal/m3u/fetcher.go` | Periodic HTTP fetch |
| `internal/store/store.go` | In-memory channel index |
| `internal/hdhr/types.go` | HDHR JSON/XML structs |
| `internal/hdhr/handler.go` | Plex discovery endpoints |
| `internal/server/server.go` | HTTP router + lifecycle |
| `internal/server/channels.go` | Clean M3U endpoint |
| `internal/epg/epg.go` | XMLTV passthrough |
| `internal/stream/streamer.go` | ffmpeg spawn + pipe |
| `internal/metrics/metrics.go` | Prometheus counters |
| `Dockerfile` | Multi-stage build with ffmpeg |
| `Makefile` | Build + multi-arch docker |
| `config.example.yaml` | Reference config |
| `test/fixtures/source.m3u` | Real fixture for parser tests |

---

## Design Decisions & Notes

- **CLI & Config:** Every setting can be set via CLI flag, environment variable (`PLEXISHOW_*`), or YAML config file. Precedence: CLI flags > env vars > config file > hardcoded defaults. The `internal/config` package handles this merging transparently.
- **No database:** In-memory `Store` is sufficient. On restart, M3U is re-fetched.
- **No ring buffer:** `io.Copy` from ffmpeg stdout directly to `http.ResponseWriter`. `Transfer-Encoding: chunked` is implicit via `Flusher`.
- **ffmpeg arguments:** `-cenc_decryption_key key_id:key` is inserted before `-i`. `-c:v copy -c:a aac` satisfies "no re-encode video, transcode audio to AAC".
- **EPG:** If `epg_url` is set, the proxy fetches it periodically and serves cached bytes at `/epg.xml`. No XML rewriting is needed if the source XMLTV already uses matching `tvg-id` values. If IDs differ, a future task can add a mapping layer.
- **Concurrency limit:** A buffered channel (`sem`) guards max concurrent ffmpeg processes. If full, `/stream/{id}` returns 503.
- **Graceful shutdown:** `signal.NotifyContext` triggers `http.Server.Shutdown`, then `stream.Manager.Shutdown()` kills all active ffmpeg processes.
- **Metrics:** Minimal Prometheus text output; no external dependency required.
- **Security:** No auth in proxy; assume reverse proxy or local network. No HTTPS.

---

**Plan complete and saved.**

Ready to execute using `subagent-driven-development` — I'll dispatch a fresh subagent per task with two-stage review (spec compliance then code quality). Shall I proceed?
