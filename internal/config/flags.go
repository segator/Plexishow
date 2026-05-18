package config

import (
	"strconv"
	"time"
)

func applyFlags(cfg *Config, flags map[string]string) {
	if flags == nil {
		return
	}
	if v, ok := flags["m3u_url"]; ok {
		cfg.M3UURL = v
	}
	if v, ok := flags["epg_url"]; ok {
		cfg.EPGURL = v
	}
	if v, ok := flags["base_url"]; ok {
		cfg.BaseURL = v
	}
	if v, ok := flags["listen_addr"]; ok {
		cfg.ListenAddr = v
	}
	if v, ok := flags["max_streams"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxStreams = n
		}
	}
	if v, ok := flags["stream_timeout"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.StreamTimeout = d
		}
	}
	if v, ok := flags["refresh_interval"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RefreshInterval = d
		}
	}
	if v, ok := flags["token"]; ok {
		cfg.DefaultHeaders.Token = v
	}
	if v, ok := flags["logs_dir"]; ok {
		cfg.LogsDir = v
	}
}
