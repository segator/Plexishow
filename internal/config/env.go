package config

import (
	"os"
	"strconv"
	"time"
)

const envPrefix = "PLEXISHOW_"

func applyEnv(cfg *Config) {
	if v := os.Getenv(envPrefix + "M3U_URL"); v != "" {
		cfg.M3UURL = v
	}
	if v := os.Getenv(envPrefix + "EPG_URL"); v != "" {
		cfg.EPGURL = v
	}
	if v := os.Getenv(envPrefix + "BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv(envPrefix + "LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv(envPrefix + "MAX_STREAMS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxStreams = n
		}
	}
	if v := os.Getenv(envPrefix + "STREAM_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.StreamTimeout = d
		}
	}
	if v := os.Getenv(envPrefix + "REFRESH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RefreshInterval = d
		}
	}
	if v := os.Getenv(envPrefix + "FFMPEG_PATH"); v != "" {
		cfg.FFmpegPath = v
	}
	if v := os.Getenv(envPrefix + "DEFAULT_HEADERS_TOKEN"); v != "" {
		cfg.DefaultHeaders.Token = v
	}
	if v := os.Getenv(envPrefix + "DEFAULT_HEADERS_REFERER"); v != "" {
		cfg.DefaultHeaders.Referer = v
	}
	if v := os.Getenv(envPrefix + "DEFAULT_HEADERS_USER_AGENT"); v != "" {
		cfg.DefaultHeaders.UserAgent = v
	}
}
