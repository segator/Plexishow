package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryIncDecActive(t *testing.T) {
	r := New()
	r.IncActive()
	r.DecActive()
	r.IncActive()
}

func TestRegistryIncErrors(t *testing.T) {
	r := New()
	r.IncErrors()
	r.IncErrors()
}

func TestHandler(t *testing.T) {
	r := New()
	r.IncActive()
	r.IncActive()
	r.IncErrors()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.Handler()(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "plexishow_active_streams 2") {
		t.Errorf("expected active_streams = 2, got body: %s", body)
	}
	if !strings.Contains(body, "plexishow_stream_errors_total 1") {
		t.Errorf("expected stream_errors_total = 1, got body: %s", body)
	}
	if !strings.Contains(body, "go_goroutines") {
		t.Error("expected go_goroutines metric")
	}
}
