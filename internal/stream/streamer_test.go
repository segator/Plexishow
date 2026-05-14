package stream

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aymerici/plexishow/internal/config"
	"github.com/aymerici/plexishow/internal/m3u"
	"github.com/aymerici/plexishow/internal/metrics"
	"github.com/aymerici/plexishow/internal/store"
)

func TestServeChannelNotFound(t *testing.T) {
	st := store.New()
	st.Replace([]m3u.Channel{
		{ID: "c1", Name: "Ch1", URL: "http://example.com/stream.mpd", KeyID: "a", Key: "b"},
	})
	cfg := config.Config{MaxStreams: 2, StreamTimeout: 5 * time.Second, FFmpegPath: "ffmpeg"}
	sm := NewManager(cfg, st, metrics.New())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stream/c2", nil)
	sm.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestConcurrencyLimit(t *testing.T) {
	st := store.New()
	st.Replace([]m3u.Channel{
		{ID: "c1", Name: "Ch1", URL: "http://example.com/a.mpd", KeyID: "a", Key: "b"},
		{ID: "c2", Name: "Ch2", URL: "http://example.com/b.mpd", KeyID: "c", Key: "d"},
	})
	cfg := config.Config{MaxStreams: 1, StreamTimeout: 1 * time.Second, FFmpegPath: "ffmpeg"}
	sm := NewManager(cfg, st, metrics.New())

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
	sm := NewManager(cfg, st, metrics.New())
	// Should not panic even if empty
	sm.Shutdown()
}

func TestBuildArgsNoKey(t *testing.T) {
	ch := m3u.Channel{URL: "http://example.com/stream.mpd"}
	args := buildArgs(ch)
	want := []string{
		"-fflags", "+discardcorrupt",
		"-headers", "",
		"-i", "http://example.com/stream.mpd",
		"-c:v", "copy",
		"-c:a", "aac",
		"-f", "mpegts",
		"-",
	}
	if len(args) != len(want) {
		t.Fatalf("expected %d args, got %d: %v", len(want), len(args), args)
	}
	for i, v := range want {
		if args[i] != v {
			t.Errorf("arg[%d] = %q, want %q", i, args[i], v)
		}
	}
}

func TestBuildArgsWithKey(t *testing.T) {
	ch := m3u.Channel{URL: "http://example.com/stream.mpd", KeyID: "kid1", Key: "key1"}
	args := buildArgs(ch)
	// -cenc_decryption_key should be inserted after -fflags +discardcorrupt and before -headers
	if len(args) == 0 {
		t.Fatal("no args")
	}
	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-cenc_decryption_key" && args[i+1] == "kid1:key1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("missing cenc_decryption_key in args: %v", args)
	}
}

func TestBuildHeaders(t *testing.T) {
	h := map[string]string{
		"Referer":    "https://tv.example.com/",
		"User-Agent": "TestAgent",
	}
	got := buildHeaders(h)
	if !strings.Contains(got, "Referer: https://tv.example.com/") {
		t.Errorf("missing Referer in %q", got)
	}
	if !strings.Contains(got, "User-Agent: TestAgent") {
		t.Errorf("missing User-Agent in %q", got)
	}
}

func TestBuildHeadersEmpty(t *testing.T) {
	got := buildHeaders(map[string]string{})
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
