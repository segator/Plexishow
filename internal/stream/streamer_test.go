package stream

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
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
	cfg := defaultTestFFmpegConfig()
	cfg.Transcode = false
	args := buildArgs(ch, cfg)
	want := []string{
		"-y",
		"-probesize", "1500000",
		"-analyzeduration", "1000000",
		"-fflags", "+igndts+genpts",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-rw_timeout", "10000000",
		"-i", "http://example.com/stream.mpd",
		"-map", "0:v:0", "-map", "0:a:0",
		"-c:v", "copy",
		"-c:a", "copy",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
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

func TestBuildArgsTranscode(t *testing.T) {
	ch := m3u.Channel{URL: "http://example.com/stream.mpd"}
	cfg := defaultTestFFmpegConfig()
	cfg.Transcode = true
	args := buildArgs(ch, cfg)
	want := []string{
		"-y",
		"-probesize", "1500000",
		"-analyzeduration", "1000000",
		"-fflags", "+igndts+genpts",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-rw_timeout", "10000000",
		"-i", "http://example.com/stream.mpd",
		"-map", "0:v:0", "-map", "0:a:0",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "18",
		"-c:a", "aac", "-b:a", "192k", "-af", "aresample=async=1",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
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

func TestBuildArgsNVENC(t *testing.T) {
	ch := m3u.Channel{URL: "http://example.com/stream.mpd"}
	cfg := defaultTestFFmpegConfig()
	cfg.Transcode = true
	cfg.HWAccel = "nvenc"
	cfg.Preset = "p4"
	args := buildArgs(ch, cfg)
	want := []string{
		"-y",
		"-probesize", "1500000",
		"-analyzeduration", "1000000",
		"-fflags", "+igndts+genpts",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-rw_timeout", "10000000",
		"-i", "http://example.com/stream.mpd",
		"-map", "0:v:0", "-map", "0:a:0",
		"-c:v", "h264_nvenc", "-preset", "p4", "-cq", "18",
		"-c:a", "aac", "-b:a", "192k", "-af", "aresample=async=1",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
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

func TestBuildArgsVAAPI(t *testing.T) {
	ch := m3u.Channel{URL: "http://example.com/stream.mpd"}
	cfg := defaultTestFFmpegConfig()
	cfg.Transcode = true
	cfg.HWAccel = "vaapi"
	cfg.VAAPIDevice = "/dev/dri/renderD129"
	args := buildArgs(ch, cfg)
	want := []string{
		"-y",
		"-vaapi_device", "/dev/dri/renderD129",
		"-probesize", "1500000",
		"-analyzeduration", "1000000",
		"-fflags", "+igndts+genpts",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-rw_timeout", "10000000",
		"-i", "http://example.com/stream.mpd",
		"-map", "0:v:0", "-map", "0:a:0",
		"-vf", "format=nv12,hwupload",
		"-c:v", "h264_vaapi", "-qp", "18",
		"-c:a", "aac", "-b:a", "192k", "-af", "aresample=async=1",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
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

func TestBuildArgsQSV(t *testing.T) {
	ch := m3u.Channel{URL: "http://example.com/stream.mpd"}
	cfg := defaultTestFFmpegConfig()
	cfg.Transcode = true
	cfg.HWAccel = "qsv"
	cfg.Preset = "veryfast"
	args := buildArgs(ch, cfg)
	want := []string{
		"-y",
		"-probesize", "1500000",
		"-analyzeduration", "1000000",
		"-fflags", "+igndts+genpts",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-rw_timeout", "10000000",
		"-i", "http://example.com/stream.mpd",
		"-map", "0:v:0", "-map", "0:a:0",
		"-c:v", "h264_qsv", "-preset", "veryfast", "-global_quality", "18",
		"-c:a", "aac", "-b:a", "192k", "-af", "aresample=async=1",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
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
	args := buildArgs(ch, defaultTestFFmpegConfig())
	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-cenc_decryption_key" && args[i+1] == "key1" {
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
	args := buildArgs(ch, defaultTestFFmpegConfig())

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

	// Referer and X-TCDN-token should be inside the single -headers string
	foundHdr := false
	var hdrValue string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-headers" {
			hdrValue = args[i+1]
			foundHdr = true
			break
		}
	}
	if !foundHdr {
		t.Fatalf("missing -headers in args: %v", args)
	}
	if !strings.Contains(hdrValue, "Referer: https://tv.example.com/") {
		t.Errorf("missing Referer in headers: %q", hdrValue)
	}
	if !strings.Contains(hdrValue, "X-TCDN-token: tok123") {
		t.Errorf("missing X-TCDN-token in headers: %q", hdrValue)
	}
}

func TestBroadcastMultiplex(t *testing.T) {
	pr, pw := io.Pipe()
	sess := &session{
		stdout: pr,
		cancel: func() {},
		subs:   make(map[chan []byte]struct{}),
		done:   make(chan struct{}),
	}

	ch1 := make(chan []byte, 10)
	ch2 := make(chan []byte, 10)
	if !sess.addSub(ch1) {
		t.Fatal("addSub returned false for fresh session")
	}
	if !sess.addSub(ch2) {
		t.Fatal("addSub returned false for fresh session")
	}

	go sess.broadcast(nil, "test", "")

	_, _ = pw.Write([]byte("hello"))
	_ = pw.Close()

	var got1, got2 string
	select {
	case b := <-ch1:
		got1 = string(b)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ch1")
	}
	select {
	case b := <-ch2:
		got2 = string(b)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ch2")
	}

	if got1 != "hello" {
		t.Errorf("ch1 got %q, want hello", got1)
	}
	if got2 != "hello" {
		t.Errorf("ch2 got %q, want hello", got2)
	}

	<-sess.done
}

func TestSessionIdleTimeout(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		t.Skip("cannot start test process:", err)
	}

	sess := &session{
		cmd:         cmd,
		stdout:      stdout,
		cancel:      func() { _ = cmd.Process.Kill() },
		subs:        make(map[chan []byte]struct{}),
		done:        make(chan struct{}),
		idleTimeout: 100 * time.Millisecond,
	}

	go sess.broadcast(stderr, "test", "")

	ch := make(chan []byte, 1)
	if !sess.addSub(ch) {
		t.Fatal("addSub returned false for fresh session")
	}
	sess.removeSub(ch)

	select {
	case <-sess.done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("session did not die after idle timeout")
	}

	if cmd.ProcessState == nil {
		t.Fatal("process did not exit")
	}
}

func TestSessionAddSubFalse(t *testing.T) {
	sess := &session{
		cancel: func() {},
		subs:   nil,
		done:   make(chan struct{}),
	}
	ch := make(chan []byte, 1)
	if sess.addSub(ch) {
		t.Fatal("addSub should return should return false when subs is nil")
	}
}

func defaultTestFFmpegConfig() config.FFmpegConfig {
	return config.FFmpegConfig{
		Probesize:         "1500000",
		Analyzeduration:   "1000000",
		Preset:            "veryfast",
		CRF:               18,
		AudioBitrate:      "192k",
		VAAPIDevice:       "/dev/dri/renderD128",
		Reconnect:         true,
		ReconnectStreamed: true,
		ReconnectDelayMax: 5,
		RWTimeout:         "10000000",
	}
}
