package m3u

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aymerici/plexishow/internal/config"
)

// ChannelStore is the interface required by Fetcher to persist channels.
type ChannelStore interface {
	Replace(chs []Channel)
}

type Fetcher struct {
	cfg    *config.Config
	store  ChannelStore
	client *http.Client
}

func NewFetcher(cfg *config.Config, s ChannelStore) *Fetcher {
	return &Fetcher{
		cfg:    cfg,
		store:  s,
		client: &http.Client{Timeout: cfg.StreamTimeout},
	}
}

func (f *Fetcher) Pull() error {
	req, err := http.NewRequestWithContext(context.Background(), "GET", f.cfg.M3UURL, nil)
	if err != nil {
		return err
	}
	if f.cfg.DefaultHeaders.Token != "" {
		req.Header.Set("X-TCDN-token", f.cfg.DefaultHeaders.Token)
	}
	if f.cfg.DefaultHeaders.Referer != "" {
		req.Header.Set("Referer", f.cfg.DefaultHeaders.Referer)
	}
	if f.cfg.DefaultHeaders.UserAgent != "" {
		req.Header.Set("User-Agent", f.cfg.DefaultHeaders.UserAgent)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch m3u: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch m3u: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read m3u: %w", err)
	}
	channels, epgURL, err := Parse(data)
	if err != nil {
		return fmt.Errorf("parse m3u: %w", err)
	}
	if f.cfg.EPGURL == "" && epgURL != "" {
		f.cfg.EPGURL = epgURL
	}

	for i := range channels {
		if channels[i].Headers == nil {
			channels[i].Headers = make(map[string]string)
		}
		if f.cfg.DefaultHeaders.Token != "" {
			channels[i].Headers["X-TCDN-token"] = f.cfg.DefaultHeaders.Token
		}
		if _, ok := channels[i].Headers["Referer"]; !ok && f.cfg.DefaultHeaders.Referer != "" {
			channels[i].Headers["Referer"] = f.cfg.DefaultHeaders.Referer
		}
		if _, ok := channels[i].Headers["User-Agent"]; !ok && f.cfg.DefaultHeaders.UserAgent != "" {
			channels[i].Headers["User-Agent"] = f.cfg.DefaultHeaders.UserAgent
		}
	}

	f.store.Replace(channels)
	return nil
}

func (f *Fetcher) Start() {
	go func() {
		ticker := time.NewTicker(f.cfg.RefreshInterval)
		defer ticker.Stop()
		_ = f.Pull()
		for range ticker.C {
			_ = f.Pull()
		}
	}()
}
