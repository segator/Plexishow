package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aymerici/plexishow/internal/config"
	"github.com/aymerici/plexishow/internal/epg"
	"github.com/aymerici/plexishow/internal/m3u"
	"github.com/aymerici/plexishow/internal/metrics"
	"github.com/aymerici/plexishow/internal/store"
	"github.com/aymerici/plexishow/internal/stream"
)

func TestRouterHealth(t *testing.T) {
	st := store.New()
	srv := New("http://localhost:8080", st, nil, nil, metrics.New())
	router := srv.Router()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != "ok" {
		t.Errorf("unexpected body: %q", w.Body.String())
	}
}

func TestRouterMetrics(t *testing.T) {
	st := store.New()
	srv := New("http://localhost:8080", st, nil, nil, metrics.New())
	router := srv.Router()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "plexishow_active_streams") {
		t.Error("missing plexishow_active_streams metric")
	}
}

func TestRouterLineup(t *testing.T) {
	st := store.New()
	st.Replace([]m3u.Channel{
		{ID: "ch1", Name: "One", TVGID: "1.1"},
	})
	srv := New("http://localhost:8080", st, nil, nil, metrics.New())
	router := srv.Router()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/lineup.json", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "One") {
		t.Error("missing channel name in lineup")
	}
}

func TestRouterDiscover(t *testing.T) {
	st := store.New()
	srv := New("http://localhost:8080", st, nil, nil, metrics.New())
	router := srv.Router()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/discover.json", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Plexishow") {
		t.Error("missing friendly name in discover")
	}
}

func TestRouterEPGNotRegisteredWhenNil(t *testing.T) {
	st := store.New()
	srv := New("http://localhost:8080", st, nil, nil, metrics.New())
	router := srv.Router()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/epg.xml", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when epg nil, got %d", w.Code)
	}
}

func TestRouterEPGRegistered(t *testing.T) {
	st := store.New()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte("<tv></tv>"))
	}))
	defer ts.Close()

	epgSrc := epg.New(ts.URL, nil)
	if err := epgSrc.Refresh(); err != nil {
		t.Fatalf("refresh epg: %v", err)
	}

	srv := New("http://localhost:8080", st, nil, epgSrc, metrics.New())
	router := srv.Router()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/epg.xml", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRouterStreamNotFound(t *testing.T) {
	st := store.New()
	cfg := config.Config{MaxStreams: 2, StreamTimeout: 1 * time.Second, FFmpegPath: "ffmpeg"}
	sm := stream.NewManager(cfg, st, metrics.New())
	srv := New("http://localhost:8080", st, sm, nil, metrics.New())
	router := srv.Router()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stream/missing", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
