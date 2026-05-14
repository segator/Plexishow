package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aymerici/plexishow/internal/config"
	"github.com/aymerici/plexishow/internal/epg"
	"github.com/aymerici/plexishow/internal/m3u"
	"github.com/aymerici/plexishow/internal/metrics"
	"github.com/aymerici/plexishow/internal/server"
	"github.com/aymerici/plexishow/internal/store"
	"github.com/aymerici/plexishow/internal/stream"
)

// version is set at build time via -ldflags "-X main.version=$(VERSION)"
var version = "dev" //nolint:unused

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	m3uURL := flag.String("m3u-url", "", "M3U playlist URL (overrides config/env)")
	epgURL := flag.String("epg-url", "", "EPG XMLTV URL (overrides config/env)")
	listenAddr := flag.String("listen-addr", "", "HTTP listen address (overrides config/env)")
	maxStreams := flag.Int("max-streams", 0, "Max concurrent streams (overrides config/env)")
	streamTimeout := flag.String("stream-timeout", "", "Per-stream idle timeout (overrides config/env)")
	refreshInterval := flag.String("refresh-interval", "", "M3U refresh interval (overrides config/env)")
	ffmpegPath := flag.String("ffmpeg-path", "", "Path to ffmpeg binary (overrides config/env)")
	token := flag.String("token", "", "X-TCDN-token header value (overrides config/env)")
	referer := flag.String("referer", "", "Referer header value (overrides config/env)")
	userAgent := flag.String("user-agent", "", "User-Agent header value (overrides config/env)")
	flag.Parse()

	flags := make(map[string]string)
	if *m3uURL != "" {
		flags["m3u_url"] = *m3uURL
	}
	if *epgURL != "" {
		flags["epg_url"] = *epgURL
	}
	if *listenAddr != "" {
		flags["listen_addr"] = *listenAddr
	}
	if *maxStreams > 0 {
		flags["max_streams"] = strconv.Itoa(*maxStreams)
	}
	if *streamTimeout != "" {
		flags["stream_timeout"] = *streamTimeout
	}
	if *refreshInterval != "" {
		flags["refresh_interval"] = *refreshInterval
	}
	if *ffmpegPath != "" {
		flags["ffmpeg_path"] = *ffmpegPath
	}
	if *token != "" {
		flags["token"] = *token
	}
	if *referer != "" {
		flags["referer"] = *referer
	}
	if *userAgent != "" {
		flags["user_agent"] = *userAgent
	}

	cfg, err := config.Load(*configPath, flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	st := store.New()

	fetcher := m3u.NewFetcher(*cfg, st)
	if err := fetcher.Pull(); err != nil {
		fmt.Fprintf(os.Stderr, "initial m3u fetch: %v\n", err)
		os.Exit(1)
	}
	fetcher.Start()

	var epgSource *epg.Source
	if cfg.EPGURL != "" {
		fmt.Println("EPG URL:", cfg.EPGURL)
		epgSource = epg.New(cfg.EPGURL, &http.Client{Timeout: 30 * time.Second})
		if err := epgSource.Refresh(); err != nil {
			fmt.Fprintf(os.Stderr, "initial epg fetch: %v\n", err)
		}
		go func() {
			ticker := time.NewTicker(cfg.RefreshInterval)
			defer ticker.Stop()
			for range ticker.C {
				_ = epgSource.Refresh()
			}
		}()
	} else {
		fmt.Println("EPG: not configured (no url-tvg in M3U, no -epg-url flag, no PLEXISHOW_EPG_URL env)")
	}

	metricsReg := metrics.New()

	streamer := stream.NewManager(*cfg, st, metricsReg)

	baseURL := fmt.Sprintf("http://%s", cfg.ListenAddr)
	srv := server.New(baseURL, st, streamer, epgSource, metricsReg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx, cfg.ListenAddr); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}

	streamer.Shutdown()
	fmt.Println("shutdown complete")
}
