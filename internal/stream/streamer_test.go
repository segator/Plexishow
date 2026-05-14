package stream

import (
	"net/http"
	"net/http/httptest"
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
		"-y",
		"-re", "-i", "http://example.com/stream.mpd",
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

func TestBuildArgsWithHeaders(t *testing.T) {
	ch := m3u.Channel{
		URL:     "http://example.com/stream.mpd",
		Headers: map[string]string{"User-Agent": "TestAgent", "Referer": "https://tv.example.com/", "X-TCDN-token": "tok123"},
	}
	args := buildArgs(ch)

	// User-Agent should become -user_agent
	foundUA := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-user_agent" && args[i+1] == "TestAgent" {
			foundUA = true
			break
		}
	}
	if !foundUA {
		t.Errorf("missing -user_agent in args: %v", args)
	}

	// Referer and X-TCDN-token should become individual -headers
	foundReferer := false
	foundToken := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-headers" {
			if args[i+1] == "Referer: https://tv.example.com/" {
				foundReferer = true
			}
			if args[i+1] == "X-TCDN-token: tok123" {
				foundToken = true
			}
		}
	}
	if !foundReferer {
		t.Errorf("missing Referer header in args: %v", args)
	}
	if !foundToken {
		t.Errorf("missing X-TCDN-token header in args: %v", args)
	}
}
