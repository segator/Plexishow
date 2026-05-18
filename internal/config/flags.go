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
	if v, ok := flags["transcode"]; ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Transcode = b
		}
	}
	if v, ok := flags["ffmpeg_probesize"]; ok {
		cfg.FFmpeg.Probesize = v
	}
	if v, ok := flags["ffmpeg_analyzeduration"]; ok {
		cfg.FFmpeg.Analyzeduration = v
	}
	if v, ok := flags["ffmpeg_transcode"]; ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.FFmpeg.Transcode = b
		}
	}
	if v, ok := flags["ffmpeg_hwaccel"]; ok {
		cfg.FFmpeg.HWAccel = v
	}
	if v, ok := flags["ffmpeg_preset"]; ok {
		cfg.FFmpeg.Preset = v
	}
	if v, ok := flags["ffmpeg_crf"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.FFmpeg.CRF = n
		}
	}
	if v, ok := flags["ffmpeg_audio_codec"]; ok {
		cfg.FFmpeg.AudioCodec = v
	}
	if v, ok := flags["ffmpeg_audio_bitrate"]; ok {
		cfg.FFmpeg.AudioBitrate = v
	}
	if v, ok := flags["ffmpeg_vaapi_device"]; ok {
		cfg.FFmpeg.VAAPIDevice = v
	}
	if v, ok := flags["ffmpeg_reconnect"]; ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.FFmpeg.Reconnect = b
		}
	}
	if v, ok := flags["ffmpeg_reconnect_streamed"]; ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.FFmpeg.ReconnectStreamed = b
		}
	}
	if v, ok := flags["ffmpeg_reconnect_delay_max"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.FFmpeg.ReconnectDelayMax = n
		}
	}
	if v, ok := flags["ffmpeg_rw_timeout"]; ok {
		cfg.FFmpeg.RWTimeout = v
	}
}
