package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aymerici/plexishow/internal/m3u"
	"github.com/aymerici/plexishow/internal/metrics"
	"github.com/aymerici/plexishow/internal/store"
)

func TestServeM3U(t *testing.T) {
	st := store.New()
	st.Replace([]m3u.Channel{
		{ID: "c1", Name: "Ch1", TVGID: "1.1", TVGLogo: "http://logo", Group: "Sports", URL: "http://src"},
	})
	srv := New("http://localhost:8080", st, nil, nil, metrics.New())
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
