package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
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

func defaultBaseURL(listenAddr string) string {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		// IPv6 or missing port fallback
		return "http://localhost" + listenAddr
	}
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	m3uURL := flag.String("m3u-url", "", "M3U playlist URL (overrides config/env)")
	epgURL := flag.String("epg-url", "", "EPG XMLTV URL (overrides config/env)")
	baseURLFlag := flag.String("base-url", "", "Base URL advertised to clients (overrides config/env)")
	listenAddr := flag.String("listen-addr", "", "HTTP listen address (overrides config/env)")
	maxStreams := flag.Int("max-streams", 0, "Max concurrent streams (overrides config/env)")
	streamTimeout := flag.String("stream-timeout", "", "Per-stream idle timeout (overrides config/env)")
	refreshInterval := flag.String("refresh-interval", "", "M3U refresh interval (overrides config/env)")
	token := flag.String("token", "", "X-TCDN-token for all channels (overrides M3U stream_headers)")
	logsDir := flag.String("logs-dir", "", "Directory to write per-channel ffmpeg logs (optional)")
	transcode := flag.Bool("transcode", false, "Enable full real-time CPU transcoding instead of direct copy")
	ffmpegProbesize := flag.String("ffmpeg-probesize", "", "FFmpeg probesize")
	ffmpegAnalyzeduration := flag.String("ffmpeg-analyzeduration", "", "FFmpeg analyzeduration")
	ffmpegTranscode := flag.Bool("ffmpeg-transcode", false, "FFmpeg enable transcoding")
	ffmpegHWAccel := flag.String("ffmpeg-hwaccel", "", "FFmpeg hardware acceleration: nvenc, vaapi, qsv")
	ffmpegPreset := flag.String("ffmpeg-preset", "", "FFmpeg encoder preset")
	ffmpegCRF := flag.Int("ffmpeg-crf", 0, "FFmpeg CRF / quality parameter")
	ffmpegAudioBitrate := flag.String("ffmpeg-audio-bitrate", "", "FFmpeg audio bitrate")
	ffmpegVAAPIDevice := flag.String("ffmpeg-vaapi-device", "", "FFmpeg VAAPI device path")
	ffmpegReconnect := flag.Bool("ffmpeg-reconnect", true, "Enable FFmpeg automatic reconnect on HTTP failures")
	ffmpegReconnectStreamed := flag.Bool("ffmpeg-reconnect-streamed", true, "Enable FFmpeg reconnect for live streams")
	ffmpegReconnectDelayMax := flag.Int("ffmpeg-reconnect-delay-max", 5, "Maximum delay between reconnect attempts")
	ffmpegRWTimeout := flag.String("ffmpeg-rw-timeout", "10000000", "Read/Write timeout in microseconds")
	flag.Parse()

	flags := make(map[string]string)
	if *m3uURL != "" {
		flags["m3u_url"] = *m3uURL
	}
	if *epgURL != "" {
		flags["epg_url"] = *epgURL
	}
	if *baseURLFlag != "" {
		flags["base_url"] = *baseURLFlag
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
	if *token != "" {
		flags["token"] = *token
	}
	if *logsDir != "" {
		flags["logs_dir"] = *logsDir
	}
	if *transcode {
		flags["transcode"] = "true"
	}
	if *ffmpegProbesize != "" {
		flags["ffmpeg_probesize"] = *ffmpegProbesize
	}
	if *ffmpegAnalyzeduration != "" {
		flags["ffmpeg_analyzeduration"] = *ffmpegAnalyzeduration
	}
	if *ffmpegTranscode {
		flags["ffmpeg_transcode"] = "true"
	}
	if *ffmpegHWAccel != "" {
		flags["ffmpeg_hwaccel"] = *ffmpegHWAccel
	}
	if *ffmpegPreset != "" {
		flags["ffmpeg_preset"] = *ffmpegPreset
	}
	if *ffmpegCRF > 0 {
		flags["ffmpeg_crf"] = strconv.Itoa(*ffmpegCRF)
	}
	if *ffmpegAudioBitrate != "" {
		flags["ffmpeg_audio_bitrate"] = *ffmpegAudioBitrate
	}
	if *ffmpegVAAPIDevice != "" {
		flags["ffmpeg_vaapi_device"] = *ffmpegVAAPIDevice
	}
	flags["ffmpeg_reconnect"] = strconv.FormatBool(*ffmpegReconnect)
	flags["ffmpeg_reconnect_streamed"] = strconv.FormatBool(*ffmpegReconnectStreamed)
	flags["ffmpeg_reconnect_delay_max"] = strconv.Itoa(*ffmpegReconnectDelayMax)
	flags["ffmpeg_rw_timeout"] = *ffmpegRWTimeout

	cfg, err := config.Load(*configPath, flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	if _, err := exec.LookPath(cfg.FFmpegPath); err != nil {
		fmt.Fprintf(os.Stderr, "ffmpeg not found: %v\n", err)
		fmt.Fprintln(os.Stderr, "Install ffmpeg and ensure it is in your PATH, or set -ffmpeg-path")
		os.Exit(1)
	}

	st := store.New()

	metricsReg := metrics.New()

	fetcher := m3u.NewFetcher(cfg, st)
	fetcher.SetMetrics(metricsReg)
	if err := fetcher.Pull(); err != nil {
		fmt.Fprintf(os.Stderr, "initial m3u fetch: %v\n", err)
		os.Exit(1)
	}
	fetcher.Start()

	var epgSource *epg.Source
	if cfg.EPGURL != "" {
		fmt.Println("EPG URL:", cfg.EPGURL)
		epgSource = epg.New(cfg.EPGURL, &http.Client{Timeout: 30 * time.Second})
		epgSource.SetMetrics(metricsReg)
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

	streamer := stream.NewManager(*cfg, st, metricsReg)

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(cfg.ListenAddr)
	}
	fmt.Println("Base URL:", baseURL)

	chs := st.All()
	fmt.Printf("Loaded %d channels\n", len(chs))
	for _, c := range chs {
		fmt.Printf("  - %s (%s) -> %s/stream/%s\n", c.Name, c.Group, baseURL, c.ID)
	}

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
