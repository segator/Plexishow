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
	os.Setenv("PLEXISHOW_BASE_URL", "http://env.com")
	defer os.Unsetenv("PLEXISHOW_M3U_URL")
	defer os.Unsetenv("PLEXISHOW_MAX_STREAMS")
	defer os.Unsetenv("PLEXISHOW_BASE_URL")

	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	data := []byte(`m3u_url: "https://file.com/playlist.m3u"
max_streams: 2
`)
	_ = os.WriteFile(p, data, 0644)

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
	if cfg.BaseURL != "http://env.com" {
		t.Errorf("env should override file: got %s", cfg.BaseURL)
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
