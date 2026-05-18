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
	if v := os.Getenv(envPrefix + "LOGS_DIR"); v != "" {
		cfg.LogsDir = v
	}
	if v := os.Getenv(envPrefix + "TRANSCODE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Transcode = b
		}
	}
	if v := os.Getenv(envPrefix + "FFMPEG_PROBESIZE"); v != "" {
		cfg.FFmpeg.Probesize = v
	}
	if v := os.Getenv(envPrefix + "FFMPEG_ANALYZEDURATION"); v != "" {
		cfg.FFmpeg.Analyzeduration = v
	}
	if v := os.Getenv(envPrefix + "FFMPEG_TRANSCODE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.FFmpeg.Transcode = b
		}
	}
	if v := os.Getenv(envPrefix + "FFMPEG_HWACCEL"); v != "" {
		cfg.FFmpeg.HWAccel = v
	}
	if v := os.Getenv(envPrefix + "FFMPEG_PRESET"); v != "" {
		cfg.FFmpeg.Preset = v
	}
	if v := os.Getenv(envPrefix + "FFMPEG_CRF"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.FFmpeg.CRF = n
		}
	}
	if v := os.Getenv(envPrefix + "FFMPEG_AUDIO_CODEC"); v != "" {
		cfg.FFmpeg.AudioCodec = v
	}
	if v := os.Getenv(envPrefix + "FFMPEG_AUDIO_BITRATE"); v != "" {
		cfg.FFmpeg.AudioBitrate = v
	}
	if v := os.Getenv(envPrefix + "FFMPEG_VAAPI_DEVICE"); v != "" {
		cfg.FFmpeg.VAAPIDevice = v
	}
	if v := os.Getenv(envPrefix + "FFMPEG_RECONNECT"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.FFmpeg.Reconnect = b
		}
	}
	if v := os.Getenv(envPrefix + "FFMPEG_RECONNECT_STREAMED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.FFmpeg.ReconnectStreamed = b
		}
	}
	if v := os.Getenv(envPrefix + "FFMPEG_RECONNECT_DELAY_MAX"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.FFmpeg.ReconnectDelayMax = n
		}
	}
	if v := os.Getenv(envPrefix + "FFMPEG_RW_TIMEOUT"); v != "" {
		cfg.FFmpeg.RWTimeout = v
	}
	if v := os.Getenv(envPrefix + "DEFAULT_HEADERS_TOKEN"); v != "" {
		cfg.DefaultHeaders.Token = v
	}
}
