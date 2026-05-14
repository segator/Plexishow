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
	DefaultHeaders  Headers       `yaml:"default_headers"`
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
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	cfg.ListenAddr = ":8080"
	cfg.MaxStreams = 4
	cfg.StreamTimeout = 30 * time.Second
	cfg.RefreshInterval = 1 * time.Hour
	cfg.FFmpegPath = "ffmpeg"
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
