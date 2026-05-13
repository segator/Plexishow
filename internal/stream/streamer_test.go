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
