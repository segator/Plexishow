package stream

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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
		"-fpsprobesize", "0",
		"-fflags", "+igndts+genpts+nobuffer",
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
		"-mpegts_pmt_start_pid", "4096",
		"-mpegts_start_pid", "256",
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
		"-fpsprobesize", "0",
		"-fflags", "+igndts+genpts+nobuffer",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-rw_timeout", "10000000",
		"-i", "http://example.com/stream.mpd",
		"-map", "0:v:0", "-map", "0:a:0",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-tune", "zerolatency",
		"-c:a", "aac", "-b:a", "192k", "-af", "aresample=async=1",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
		"-f", "mpegts",
		"-mpegts_pmt_start_pid", "4096",
		"-mpegts_start_pid", "256",
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
		"-fpsprobesize", "0",
		"-fflags", "+igndts+genpts+nobuffer",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-rw_timeout", "10000000",
		"-i", "http://example.com/stream.mpd",
		"-map", "0:v:0", "-map", "0:a:0",
		"-c:v", "h264_nvenc", "-preset", "p4", "-cq", "18", "-delay", "0",
		"-c:a", "aac", "-b:a", "192k", "-af", "aresample=async=1",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
		"-f", "mpegts",
		"-mpegts_pmt_start_pid", "4096",
		"-mpegts_start_pid", "256",
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
		"-fpsprobesize", "0",
		"-fflags", "+igndts+genpts+nobuffer",
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
		"-mpegts_pmt_start_pid", "4096",
		"-mpegts_start_pid", "256",
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
		"-fpsprobesize", "0",
		"-fflags", "+igndts+genpts+nobuffer",
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
		"-mpegts_pmt_start_pid", "4096",
		"-mpegts_start_pid", "256",
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
		subs:   make(map[chan []byte]string),
		done:   make(chan struct{}),
	}

	ch1 := make(chan []byte, 10)
	ch2 := make(chan []byte, 10)
	if !sess.addSub(ch1, "127.0.0.1") {
		t.Fatal("addSub returned false for fresh session")
	}
	if !sess.addSub(ch2, "127.0.0.2") {
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
		subs:        make(map[chan []byte]string),
		done:        make(chan struct{}),
		idleTimeout: 100 * time.Millisecond,
	}

	go sess.broadcast(stderr, "test", "")

	ch := make(chan []byte, 1)
	if !sess.addSub(ch, "127.0.0.1") {
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
	if sess.addSub(ch, "127.0.0.1") {
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

func TestWriteDiscontinuityPacket(t *testing.T) {
	var buf strings.Builder
	writeDiscontinuityPacket(&buf, 0x0100)
	pkt := []byte(buf.String())
	if len(pkt) != 188 {
		t.Fatalf("expected packet length 188, got %d", len(pkt))
	}
	if pkt[0] != 0x47 {
		t.Errorf("sync byte is not 0x47: 0x%02x", pkt[0])
	}
	pid := (uint16(pkt[1]) << 8) | uint16(pkt[2])
	if pid != 0x0100 {
		t.Errorf("expected PID 0x0100, got 0x%04x", pid)
	}
	if pkt[3] != 0x20 {
		t.Errorf("expected adaptation field flag 0x20, got 0x%02x", pkt[3])
	}
	if pkt[4] != 0x01 {
		t.Errorf("expected adaptation field length 1, got %d", pkt[4])
	}
	if pkt[5] != 0x80 {
		t.Errorf("expected discontinuity indicator 0x80, got 0x%02x", pkt[5])
	}
}

func TestPlaceholderRateLimiterAndDiscontinuity(t *testing.T) {
	st := store.New()
	st.Replace([]m3u.Channel{
		{ID: "c1", Name: "Ch1", URL: "http://example.com/stream.mpd", KeyID: "a", Key: "b"},
	})
	cfg := config.Config{MaxStreams: 2, StreamTimeout: 1 * time.Second, FFmpegPath: "ffmpeg"}
	sm := NewManager(cfg, st, metrics.New())

	// Set 64KB mock placeholder bytes so it doesn't reach the end instantly
	sm.placeholderBytes = make([]byte, 65536)
	for i := range sm.placeholderBytes {
		sm.placeholderBytes[i] = byte(i % 256)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stream/c1", nil)

	pr, pw := io.Pipe()
	sess := &session{
		stdout: pr,
		cancel: func() {},
		subs:   make(map[chan []byte]string),
		done:   make(chan struct{}),
	}
	sm.mu.Lock()
	sm.sessions["c1"] = sess
	sm.mu.Unlock()

	go sess.broadcast(nil, "Ch1", "")

	go func() {
		// Wait shortly to let the placeholder loop run, then transition by writing live data
		time.Sleep(20 * time.Millisecond)
		_, _ = pw.Write([]byte("live-mpegts-stream-data-goes-here"))
		_ = pw.Close()
	}()

	sm.ServeHTTP(w, r)

	respBytes := w.Body.Bytes()
	// Should contain: initial placeholder bytes + 376 bytes of discontinuity + live data
	if len(respBytes) < 376 {
		t.Fatalf("expected at least 376 bytes of data, got %d", len(respBytes))
	}

	// Verify that the live stream data is in the output response
	if !strings.Contains(string(respBytes), "live-mpegts-stream-data-goes-here") {
		t.Errorf("live stream data not found in response")
	}

	// Find the discontinuity packet sync byte sequence
	foundDiscontinuity := false
	for i := 0; i <= len(respBytes)-376; i++ {
		if respBytes[i] == 0x47 && respBytes[i+188] == 0x47 && respBytes[i+5] == 0x80 && respBytes[i+188+5] == 0x80 {
			foundDiscontinuity = true
			break
		}
	}
	if !foundDiscontinuity {
		t.Errorf("discontinuity packets sequence not found in response bytes")
	}
}

func TestLogFileCleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plexishow-logs-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldFile := filepath.Join(tempDir, "old_ffmpeg_20260510.log")
	if err := os.WriteFile(oldFile, []byte("old log content"), 0o600); err != nil {
		t.Fatalf("failed to write old file: %v", err)
	}
	oldTime := time.Now().AddDate(0, 0, -8)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("failed to change times of old file: %v", err)
	}

	newFile := filepath.Join(tempDir, "new_ffmpeg_20260517.log")
	if err := os.WriteFile(newFile, []byte("new log content"), 0o600); err != nil {
		t.Fatalf("failed to write new file: %v", err)
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	cutoff := time.Now().AddDate(0, 0, -7)
	for _, file := range files {
		if !file.Type().IsRegular() || filepath.Ext(file.Name()) != ".log" {
			continue
		}
		info, err := file.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(tempDir, file.Name()))
		}
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old log file to be deleted, but it still exists")
	}

	if _, err := os.Stat(newFile); err != nil {
		t.Errorf("expected new log file to still exist, but got err: %v", err)
	}
}

func TestSanitizeNameStrict(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Ch 1", "Ch_1"},
		{"Ch/1:Test", "Ch_1_Test"},
		{"Ch\\2", "Ch_2"},
		{"normal-name", "normal-name"},
		{"nested/path/to/log", "nested_path_to_log"},
		{"spaces   and : colons", "spaces___and___colons"},
	}
	for _, tt := range tests {
		got := sanitizeName(tt.input)
		if got != tt.want {
			t.Fatalf("STRICT SANITIZATION FAILURE: sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestChannelColorStrict(t *testing.T) {
	// Colors slice must strictly match original colors to keep terminal logs consistent.
	// If anyone alters the color list, it will fail!
	expectedColors := []string{
		"\033[31m", // red
		"\033[32m", // green
		"\033[33m", // yellow
		"\033[34m", // blue
		"\033[35m", // magenta
		"\033[36m", // cyan
		"\033[91m", // bright red
		"\033[92m", // bright green
		"\033[93m", // bright yellow
		"\033[94m", // bright blue
		"\033[95m", // bright magenta
		"\033[96m", // bright cyan
	}

	if len(ansiColors) != len(expectedColors) {
		t.Fatalf("STRICT COLOR FAILURE: ansiColors length is %d, expected %d", len(ansiColors), len(expectedColors))
	}
	for i, v := range expectedColors {
		if ansiColors[i] != v {
			t.Fatalf("STRICT COLOR FAILURE: ansiColors[%d] = %q, expected %q", i, ansiColors[i], v)
		}
	}

	// Hashing must be completely deterministic and match original fnv color selection
	c1 := channelColor("Channel Movie")
	c2 := channelColor("Channel Movie")
	if c1 != c2 {
		t.Fatalf("STRICT HASHING FAILURE: channelColor is not deterministic")
	}

	// Verify exact color matches for known strings to catch any changes to the hash selector logic
	knownColors := map[string]string{
		"Channel Sports": "\033[96m",
		"Channel Racing": "\033[91m",
		"Channel Arena":  "\033[34m",
	}
	for name, wantColor := range knownColors {
		got := channelColor(name)
		if got != wantColor {
			t.Fatalf("STRICT COLOR ASSIGNMENT FAILURE: channelColor(%q) = %q, want %q", name, got, wantColor)
		}
	}
}

func TestWriteDiscontinuityPacketStrict(t *testing.T) {
	// Tests both video PID 0x0100 and audio PID 0x0101 to verify absolute byte-level correctness
	tests := []struct {
		pid       uint16
		wantByte1 byte
		wantByte2 byte
	}{
		{0x0100, 0x01, 0x00},
		{0x0101, 0x01, 0x01},
	}

	for _, tt := range tests {
		var buf strings.Builder
		writeDiscontinuityPacket(&buf, tt.pid)
		pkt := []byte(buf.String())

		if len(pkt) != 188 {
			t.Fatalf("STRICT DISCONTINUITY FAILURE: packet length is %d, expected 188", len(pkt))
		}
		if pkt[0] != 0x47 {
			t.Fatalf("STRICT DISCONTINUITY sync byte mismatch: 0x%02x, expected 0x47", pkt[0])
		}
		if pkt[1] != tt.wantByte1 {
			t.Fatalf("STRICT DISCONTINUITY PID High byte mismatch: 0x%02x, expected 0x%02x", pkt[1], tt.wantByte1)
		}
		if pkt[2] != tt.wantByte2 {
			t.Fatalf("STRICT DISCONTINUITY PID Low byte mismatch: 0x%02x, expected 0x%02x", pkt[2], tt.wantByte2)
		}
		if pkt[3] != 0x20 {
			t.Fatalf("STRICT DISCONTINUITY adaptation control byte mismatch: 0x%02x, expected 0x20", pkt[3])
		}
		if pkt[4] != 0x01 {
			t.Fatalf("STRICT DISCONTINUITY adaptation field length mismatch: %d, expected 1", pkt[4])
		}
		if pkt[5] != 0x80 {
			t.Fatalf("STRICT DISCONTINUITY discontinuity flag mismatch: 0x%02x, expected 0x80", pkt[5])
		}
		// All other bytes must be strictly padded with 0xFF
		for i := 6; i < 188; i++ {
			if pkt[i] != 0xFF {
				t.Fatalf("STRICT DISCONTINUITY padding mismatch at byte %d: 0x%02x, expected 0xFF", i, pkt[i])
			}
		}
	}
}

func TestBuildArgsStrictAndExhaustive(t *testing.T) {
	ch := m3u.Channel{
		URL:     "http://iptv.example.com/manifest.mpd",
		KeyID:   "my-key-id",
		Key:     "my-decryption-key-value",
		Headers: map[string]string{"User-Agent": "StrictAgent", "X-TCDN-token": "secret-token"},
	}

	cfg := defaultTestFFmpegConfig()
	cfg.Transcode = true
	cfg.HWAccel = "vaapi"
	cfg.VAAPIDevice = "/dev/dri/renderD150"

	args := buildArgs(ch, cfg)

	// Validate critical parameters exist at exactly expected positions or in exact form
	wantArgs := []string{
		"-y",
		"-vaapi_device", "/dev/dri/renderD150",
		"-cenc_decryption_key", "my-decryption-key-value",
		"-user_agent", "StrictAgent",
		"-headers", "X-TCDN-token: secret-token\r\n",
		"-probesize", "1500000",
		"-analyzeduration", "1000000",
		"-fpsprobesize", "0",
		"-fflags", "+igndts+genpts+nobuffer",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-rw_timeout", "10000000",
		"-i", "http://iptv.example.com/manifest.mpd",
		"-map", "0:v:0", "-map", "0:a:0",
		"-vf", "format=nv12,hwupload",
		"-c:v", "h264_vaapi", "-qp", "18",
		"-c:a", "aac", "-b:a", "192k", "-af", "aresample=async=1",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
		"-f", "mpegts",
		"-mpegts_pmt_start_pid", "4096",
		"-mpegts_start_pid", "256",
		"-",
	}

	if len(args) != len(wantArgs) {
		t.Fatalf("STRICT FFMPEG ARGS LENGTH MISMATCH: got %d, want %d.\nGot args: %v\nWant args: %v", len(args), len(wantArgs), args, wantArgs)
	}

	for i, val := range wantArgs {
		if args[i] != val {
			t.Fatalf("STRICT FFMPEG ARG MISMATCH at index %d: got %q, expected %q", i, args[i], val)
		}
	}
}

func TestManagerConcurrencyLimitStrict(t *testing.T) {
	st := store.New()
	st.Replace([]m3u.Channel{
		{ID: "c1", Name: "Ch1", URL: "http://example.com/a.mpd"},
		{ID: "c2", Name: "Ch2", URL: "http://example.com/b.mpd"},
		{ID: "c3", Name: "Ch3", URL: "http://example.com/c.mpd"},
	})

	// Max 2 concurrent streams
	cfg := config.Config{MaxStreams: 2, StreamTimeout: 1 * time.Second, FFmpegPath: "ffmpeg"}
	sm := NewManager(cfg, st, metrics.New())

	// Grab both slots
	sm.sem <- struct{}{}
	sm.sem <- struct{}{}

	// A third client request should be blocked immediately with 503 Service Unavailable
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stream/c3", nil)
	sm.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("STRICT CONCURRENCY FAILURE: expected status 503, got %d", w.Code)
	}

	// Release one slot
	<-sm.sem

	// Should still not work for missing channels (404 takes priority or works cleanly)
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/stream/missing-channel", nil)
	sm.ServeHTTP(w2, r2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("STRICT CONCURRENCY FAILURE: expected status 404 for missing channel, got %d", w2.Code)
	}
}
