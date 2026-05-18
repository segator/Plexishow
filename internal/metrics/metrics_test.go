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
	r.SetM3UChannels(42)
	r.SetLastRefresh(12345678)
	r.SetEPGLastRefresh(87654321)

	r.AddViewer("M+ Cine")
	r.AddViewer("M+ Cine")
	r.RemoveViewer("M+ Cine")

	r.AddBytesSent("M+ Cine", 5000)
	r.AddBytesSent("M+ Cine", 2400)

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
	if !strings.Contains(body, "plexishow_m3u_channels_total 42") {
		t.Errorf("expected m3u_channels_total = 42, got body: %s", body)
	}
	if !strings.Contains(body, "plexishow_m3u_last_refresh_timestamp_seconds 12345678") {
		t.Errorf("expected m3u last refresh = 12345678, got body: %s", body)
	}
	if !strings.Contains(body, "plexishow_epg_last_refresh_timestamp_seconds 87654321") {
		t.Errorf("expected epg last refresh = 87654321, got body: %s", body)
	}
	if !strings.Contains(body, "plexishow_channel_viewers{channel=\"M+ Cine\"} 1") {
		t.Errorf("expected channel viewers for M+ Cine = 1, got body: %s", body)
	}
	if !strings.Contains(body, "plexishow_channel_bytes_sent_total{channel=\"M+ Cine\"} 7400") {
		t.Errorf("expected channel bytes sent for M+ Cine = 7400, got body: %s", body)
	}
	if !strings.Contains(body, "go_goroutines") {
		t.Error("expected go_goroutines metric")
	}
}
