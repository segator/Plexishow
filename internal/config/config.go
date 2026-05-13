package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	M3UURL          string        `yaml:"m3u_url"`
	EPGURL          string        `yaml:"epg_url"`
	BaseURL         string        `yaml:"base_url"`
	ListenAddr      string        `yaml:"listen_addr"`
	MaxStreams      int           `yaml:"max_streams"`
	StreamTimeout   time.Duration `yaml:"stream_timeout"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	FFmpegPath      string        `yaml:"ffmpeg_path"`
	LogsDir         string        `yaml:"logs_dir"`
	Transcode       bool          `yaml:"transcode"`
	DefaultHeaders  Headers       `yaml:"default_headers"`
	FFmpeg          FFmpegConfig  `yaml:"ffmpeg"`
}

type FFmpegConfig struct {
	Probesize         string `yaml:"probesize"`
	Analyzeduration   string `yaml:"analyzeduration"`
	Transcode         bool   `yaml:"transcode"`
	HWAccel           string `yaml:"hwaccel"`
	Preset            string `yaml:"preset"`
	CRF               int    `yaml:"crf"`
	AudioCodec        string `yaml:"audio_codec"`
	AudioBitrate      string `yaml:"audio_bitrate"`
	VAAPIDevice       string `yaml:"vaapi_device"`
	Reconnect         bool   `yaml:"reconnect"`
	ReconnectStreamed bool   `yaml:"reconnect_streamed"`
	ReconnectDelayMax int    `yaml:"reconnect_delay_max"`
	RWTimeout         string `yaml:"rw_timeout"`
}

type Headers struct {
	Token     string `yaml:"token"`
	Referer   string `yaml:"referer"`
	UserAgent string `yaml:"user_agent"`
}

func Load(filePath string, flags map[string]string) (*Config, error) {
	cfg := &Config{}
	applyDefaults(cfg)

	if filePath != "" {
		if err := loadFromFile(filePath, cfg); err != nil {
			return nil, err
		}
	}

	applyEnv(cfg)
	applyFlags(cfg, flags)

	// Sync root-level transcode with nested FFmpeg.Transcode for backward compatibility
	if cfg.Transcode {
		cfg.FFmpeg.Transcode = true
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	cfg.ListenAddr = ":8080"
	cfg.MaxStreams = 4
	cfg.StreamTimeout = 30 * time.Second
	cfg.RefreshInterval = 5 * time.Minute
	cfg.FFmpegPath = "ffmpeg"
	cfg.LogsDir = "/tmp/plexishow-logs"
	cfg.FFmpeg.Probesize = "500000"
	cfg.FFmpeg.Analyzeduration = "500000"
	cfg.FFmpeg.Preset = "veryfast"
	cfg.FFmpeg.CRF = 18
	cfg.FFmpeg.AudioCodec = "aac"
	cfg.FFmpeg.AudioBitrate = "192k"
	cfg.FFmpeg.VAAPIDevice = "/dev/dri/renderD128"
	cfg.FFmpeg.Reconnect = true
	cfg.FFmpeg.ReconnectStreamed = true
	cfg.FFmpeg.ReconnectDelayMax = 5
	cfg.FFmpeg.RWTimeout = "10000000"
}

func loadFromFile(path string, cfg *Config) error {
	//#nosec G304 -- path comes from CLI flag, intentional
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}
	return yaml.Unmarshal(b, cfg)
}
