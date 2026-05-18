package stream

import (
	"bufio"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aymerici/plexishow/internal/config"
	"github.com/aymerici/plexishow/internal/m3u"
	"github.com/aymerici/plexishow/internal/metrics"
	"github.com/aymerici/plexishow/internal/store"
)

var ansiColors = []string{
	"\033[31m", // red
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[34m", // blue
	"\033[35m", // magenta
	"\033[36m", // cyan
	"\033[91m", // bright red
	"\033[92m", // bright green
	"\033[93m", // bright yellow
	"\033[94m", // bright blue
	"\033[95m", // bright magenta
	"\033[96m", // bright cyan
}

const ansiReset = "\033[0m"

func channelColor(name string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return ansiColors[int(h.Sum32())%len(ansiColors)]
}

const defaultIdleTimeout = 30 * time.Second

type session struct {
	cmd         *exec.Cmd
	stdout      io.ReadCloser
	cancel      context.CancelFunc
	mu          sync.Mutex
	subs        map[chan []byte]string
	done        chan struct{}
	idleTimer   *time.Timer
	idleTimeout time.Duration
	hasData     bool
}

func (s *session) getHasData() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasData
}

func (s *session) addSub(ch chan []byte, ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subs == nil {
		return false
	}
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
	s.subs[ch] = ip
	return true
}

func (s *session) removeSub(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subs == nil {
		return
	}
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
	if len(s.subs) == 0 && s.idleTimer == nil {
		timeout := s.idleTimeout
		if timeout == 0 {
			timeout = defaultIdleTimeout
		}
		s.idleTimer = time.AfterFunc(timeout, func() {
			s.kill()
		})
	}
}

func (s *session) kill() {
	s.cancel()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}

func sanitizeName(name string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return r.Replace(name)
}

func (s *session) broadcast(stderr io.ReadCloser, name string, logDir string) {
	defer close(s.done)
	if stderr != nil {
		go func() {
			color := channelColor(name)
			var logFile *os.File
			if logDir != "" {
				if err := os.MkdirAll(logDir, 0o700); err == nil {
					path := filepath.Join(logDir, sanitizeName(name)+".log")
					//#nosec G304 G703 -- name is sanitized
					f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
					if err == nil {
						logFile = f
						fmt.Printf("[stream] ffmpeg log for %s → %s\n", name, path)
					}
				}
			}
			defer func() {
				if logFile != nil {
					_ = logFile.Close()
				}
			}()
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				// Filter out harmless CENC seek warnings printed every segment to keep logs clean and save disk I/O
				if strings.Contains(line, "Failed to seek for auxiliary info") {
					continue
				}
				fmt.Printf("%s[%s]%s %s\n", color, name, ansiReset, line)
				if logFile != nil {
					_, _ = fmt.Fprintln(logFile, line)
				}
			}
		}()
	}
	buf := make([]byte, 32*1024)
	for {
		n, err := s.stdout.Read(buf)
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			s.mu.Lock()
			if !s.hasData {
				s.hasData = true
			}
			for ch := range s.subs {
				select {
				case ch <- data:
				default:
					// Buffer full: skip this packet for this client.
					// For live TV a momentary glitch is better than disconnecting.
				}
			}
			s.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
	if s.stdout != nil {
		_ = s.stdout.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	if s.cmd != nil {
		_ = s.cmd.Wait()
	}
	s.mu.Lock()
	for ch := range s.subs {
		close(ch)
	}
	s.subs = nil
	s.mu.Unlock()
}

type Manager struct {
	cfg              config.Config
	store            *store.Store
	metrics          *metrics.Registry
	mu               sync.Mutex
	sessions         map[string]*session
	sem              chan struct{}
	placeholderBytes []byte
}

func NewManager(cfg config.Config, st *store.Store, metrics *metrics.Registry) *Manager {
	m := &Manager{
		cfg:      cfg,
		store:    st,
		metrics:  metrics,
		sessions: make(map[string]*session),
		sem:      make(chan struct{}, cfg.MaxStreams),
	}

	// Try to load pre-rendered retro countdown placeholder video
	placeholderPath := "assets/placeholder.ts"
	if data, err := os.ReadFile(placeholderPath); err == nil {
		m.placeholderBytes = data
		fmt.Printf("[stream] Loaded %d bytes of retro countdown placeholder video from %s\n", len(data), placeholderPath)
	} else {
		fmt.Printf("[stream] No placeholder video found at %s (skipping loading placeholder feature): %v\n", placeholderPath, err)
	}

	go m.statsLogger()
	return m
}

func (m *Manager) statsLogger() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		if len(m.sessions) > 0 {
			stats := make([]string, 0, len(m.sessions))
			for id, sess := range m.sessions {
				ch, ok := m.store.Get(id)
				name := id
				if ok {
					name = ch.Name
				}
				sess.mu.Lock()
				ips := make([]string, 0, len(sess.subs))
				for _, ip := range sess.subs {
					ips = append(ips, ip)
				}
				sess.mu.Unlock()
				if len(ips) > 0 {
					stats = append(stats, fmt.Sprintf("%s (%d users: %s)", name, len(ips), strings.Join(ips, ", ")))
				} else {
					stats = append(stats, fmt.Sprintf("%s (0 users)", name))
				}
			}
			fmt.Printf("\033[1;36m[stats] %d active streams:\033[0m %s\n", len(m.sessions), strings.Join(stats, ", "))
		}
		m.mu.Unlock()
	}
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/stream/")
	ch, ok := m.store.Get(id)
	if !ok {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}

	clientIP := r.RemoteAddr
	// Try to get real IP if behind proxy
	if realIP := r.Header.Get("X-Forwarded-For"); realIP != "" {
		clientIP = strings.Split(realIP, ",")[0]
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = realIP
	}

	fmt.Printf("[stream] user connected to %s from %s\n", ch.Name, clientIP)
	m.metrics.AddViewer(ch.Name)
	defer func() {
		fmt.Printf("[stream] user disconnected from %s from %s\n", ch.Name, clientIP)
		m.metrics.RemoveViewer(ch.Name)
	}()

	chData := make(chan []byte, 5000) // Much larger buffer for network jitter

	m.mu.Lock()
	sess, exists := m.sessions[id]
	m.mu.Unlock()

	if !exists || !sess.addSub(chData, clientIP) {
		// Session died between the unlock and addSub (or didn't exist)
		select {
		case m.sem <- struct{}{}:
		default:
			http.Error(w, "max concurrent streams reached", http.StatusServiceUnavailable)
			return
		}
		var err error
		sess, err = m.startSession(ch)
		if err != nil {
			<-m.sem
			m.metrics.IncErrors()
			http.Error(w, "failed to start stream", http.StatusInternalServerError)
			return
		}
		sess.addSub(chData, clientIP)
	}
	defer sess.removeSub(chData)

	w.Header().Set("Content-Type", "video/mp2t")
	w.WriteHeader(http.StatusOK)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}	// Play retro countdown loading placeholder if the stream hasn't produced any video frames yet
	if (sess == nil || !sess.getHasData()) && len(m.placeholderBytes) > 0 {
		fmt.Printf("[stream] playing placeholder countdown for %s\n", ch.Name)
		placeholderOffset := 0
		placeholderLen := len(m.placeholderBytes)

		// Calculate exact real-time chunk size for 100ms sleep interval
		// (duration of placeholder is 30 seconds)
		realTimeRate := placeholderLen / 30
		chunkSize := realTimeRate / 10
		if chunkSize < 1024 {
			chunkSize = 1024
		}

		for {
			if sess != nil && sess.getHasData() {
				fmt.Printf("[stream] transitioning to live stream for %s\n", ch.Name)

				// Write discontinuity packets to force VLC/Plex to reset its PTS and continue seamlessly
				writeDiscontinuityPacket(w, 0x0100) // Video stream PID
				writeDiscontinuityPacket(w, 0x0101) // Audio stream PID
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				break
			}

			select {
			case <-r.Context().Done():
				return
			default:
			}

			// Write a chunk of the placeholder
			activeChunkSize := chunkSize
			if placeholderOffset+activeChunkSize > placeholderLen {
				activeChunkSize = placeholderLen - placeholderOffset
			}

			n, err := w.Write(m.placeholderBytes[placeholderOffset : placeholderOffset+activeChunkSize])
			if err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			m.metrics.AddBytesSent(ch.Name, n)

			placeholderOffset += activeChunkSize
			if placeholderOffset >= placeholderLen {
				// If we reached the end of the 30-second placeholder video and still no live stream data,
				// it means sintonization has failed or the process died. Disconnect the client to release resources.
				fmt.Printf("[stream] loading failed after 30s for %s, disconnecting client\n", ch.Name)
				return
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	timer := time.NewTimer(m.cfg.StreamTimeout)
	defer timer.Stop()

	for {
		select {
		case data, ok := <-chData:
			if !ok {
				return
			}
			n, err := w.Write(data)
			if n > 0 {
				m.metrics.AddBytesSent(ch.Name, n)
			}
			if err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(m.cfg.StreamTimeout)
		case <-r.Context().Done():
			return
		case <-timer.C:
			return
		}
	}
}

func (m *Manager) startSession(ch m3u.Channel) (*session, error) {
	ctx, cancel := context.WithCancel(context.Background())

	args := buildArgs(ch, m.cfg.FFmpeg)
	fmt.Printf("[stream] %s: %s %s\n", ch.Name, m.cfg.FFmpegPath, strings.Join(args, " "))
	//#nosec G204 -- ffmpeg path from config, intentional
	cmd := exec.CommandContext(ctx, m.cfg.FFmpegPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		cancel()
		_ = stdout.Close()
		if stderr != nil {
			_ = stderr.Close()
		}
		return nil, err
	}
	m.metrics.IncActive()

	sess := &session{
		cmd:         cmd,
		stdout:      stdout,
		cancel:      cancel,
		subs:        make(map[chan []byte]string),
		done:        make(chan struct{}),
		idleTimeout: defaultIdleTimeout,
	}

	m.mu.Lock()
	m.sessions[ch.ID] = sess
	m.mu.Unlock()

	go func() {
		sess.broadcast(stderr, ch.Name, m.cfg.LogsDir)
		m.mu.Lock()
		delete(m.sessions, ch.ID)
		m.mu.Unlock()
		<-m.sem
		m.metrics.DecActive()
		fmt.Printf("[stream] closed stream %s\n", ch.Name)
	}()

	return sess, nil
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sess := range m.sessions {
		sess.kill()
	}
}

func buildArgs(ch m3u.Channel, cfg config.FFmpegConfig) []string {
	var args []string

	// -y: Automatically overwrite output files without prompting (critical for non-interactive execution)
	args = append(args, "-y")

	// VAAPI requires -vaapi_device to be defined BEFORE -i (global option)
	if cfg.Transcode && cfg.HWAccel == "vaapi" {
		device := cfg.VAAPIDevice
		if device == "" {
			device = "/dev/dri/renderD128"
		}
		args = append(args, "-vaapi_device", device)
	}

	// -cenc_decryption_key: The 128-bit hex key to decrypt AES-CTR Common Encryption (CENC) streams
	if ch.Key != "" {
		args = append(args, "-cenc_decryption_key", ch.Key)
	}

	// -user_agent: Custom HTTP User-Agent header expected by the IPTV CDN/origin server
	if ua, ok := ch.Headers["User-Agent"]; ok {
		args = append(args, "-user_agent", ua)
	}

	// Build a single -headers string with all headers separated by \r\n
	var hdr strings.Builder
	for k, v := range ch.Headers {
		if k == "User-Agent" {
			continue
		}
		fmt.Fprintf(&hdr, "%s: %s\r\n", k, v)
	}
	if hdr.Len() > 0 {
		// -headers: Dynamic custom HTTP headers (such as Authorization/X-TCDN-token and Referer)
		args = append(args, "-headers", hdr.String())
	}

	// Reduce startup analysis time
	probesize := cfg.Probesize
	if probesize == "" {
		probesize = "1500000"
	}
	analyzeduration := cfg.Analyzeduration
	if analyzeduration == "" {
		analyzeduration = "1000000"
	}
	// -probesize: Size in bytes of input to analyze for stream information. A lower size speeds up startup
	args = append(args, "-probesize", probesize)
	// -analyzeduration: Duration in microseconds to analyze for stream information. A lower duration speeds up startup
	args = append(args, "-analyzeduration", analyzeduration)
	// -fpsprobesize 0: Do not read extra packets to estimate frame rate of secondary tracks, stopping DASH startup delays
	args = append(args, "-fpsprobesize", "0")
	// -fflags +igndts+genpts+nobuffer: +igndts ignores invalid DTS timestamps; +genpts generates missing PTS; +nobuffer disables internal demuxer buffering for minimal latency
	args = append(args, "-fflags", "+igndts+genpts+nobuffer")

	if cfg.Reconnect {
		// -reconnect 1: Retry HTTP connections automatically on transient network drops
		args = append(args, "-reconnect", "1")
	}
	if cfg.ReconnectStreamed {
		// -reconnect_streamed 1: Force reconnection support even for streamed HTTP payloads (like DASH chunks)
		args = append(args, "-reconnect_streamed", "1")
	}
	if cfg.ReconnectDelayMax > 0 {
		// -reconnect_delay_max: Maximum backoff delay in seconds between connection retries
		args = append(args, "-reconnect_delay_max", strconv.Itoa(cfg.ReconnectDelayMax))
	}
	if cfg.RWTimeout != "" {
		// -rw_timeout: Read/Write network operation timeout in microseconds to fail fast and trigger reconnects
		args = append(args, "-rw_timeout", cfg.RWTimeout)
	}

	// -i: The input stream URL (e.g., DASH .mpd or HLS .m3u8)
	args = append(args, "-i", ch.URL)
	// -map 0:v:0 -map 0:a:0: Selects only the first video track and the first audio track to skip secondary tracks
	args = append(args, "-map", "0:v:0", "-map", "0:a:0")

	if cfg.Transcode {
		// Video Transcoding
		switch cfg.HWAccel {
		case "nvenc":
			// GPU Transcoding with NVIDIA NVENC (-c:v h264_nvenc)
			args = append(args, "-c:v", "h264_nvenc")
			if cfg.Preset != "" {
				args = append(args, "-preset", cfg.Preset)
			} else {
				args = append(args, "-preset", "p4")
			}
			if cfg.CRF > 0 {
				args = append(args, "-cq", strconv.Itoa(cfg.CRF))
			} else {
				args = append(args, "-cq", "18")
			}
			// -delay 0: Force zero-latency encoding mode in the NVENC hardware chip (no frame lookahead delay).
			// This option is exclusive to NVENC and guarantees instantaneous frame delivery without buffering.
			args = append(args, "-delay", "0")
		case "vaapi":
			// GPU Transcoding with Intel/AMD VAAPI H.264, upload input frames to VAAPI surfaces on GPU
			args = append(args, "-vf", "format=nv12,hwupload")
			args = append(args, "-c:v", "h264_vaapi")
			if cfg.CRF > 0 {
				args = append(args, "-qp", strconv.Itoa(cfg.CRF))
			} else {
				args = append(args, "-qp", "18")
			}
		case "qsv":
			// GPU Transcoding with Intel QSV H.264
			args = append(args, "-c:v", "h264_qsv")
			if cfg.Preset != "" {
				args = append(args, "-preset", cfg.Preset)
			}
			if cfg.CRF > 0 {
				args = append(args, "-global_quality", strconv.Itoa(cfg.CRF))
			} else {
				args = append(args, "-global_quality", "18")
			}
		default: // CPU encoding using libx264
			// CPU Transcoding with standard x264 (-c:v libx264)
			args = append(args, "-c:v", "libx264")
			preset := cfg.Preset
			if preset == "" {
				preset = "veryfast"
			}
			args = append(args, "-preset", preset)
			crf := cfg.CRF
			if crf == 0 {
				crf = 18
			}
			args = append(args, "-crf", strconv.Itoa(crf))
			// -tune zerolatency: Disables B-frames and lookahead to achieve sub-frame encoding delay on CPU
			args = append(args, "-tune", "zerolatency")
		}

		// Audio Transcoding
		audioCodec := cfg.AudioCodec
		if audioCodec == "" {
			audioCodec = "aac"
		}
		args = append(args, "-c:a", audioCodec)
		audioBitrate := cfg.AudioBitrate
		if audioBitrate == "" {
			audioBitrate = "192k"
		}
		args = append(args, "-b:a", audioBitrate)
		args = append(args, "-af", "aresample=async=1")
	} else {
		// Direct Stream Copy (No transcoding): extremely low CPU load, copy raw streams as is
		args = append(args, "-c:v", "copy", "-c:a", "copy")
	}

	// -avoid_negative_ts make_zero: Shift all timestamps so they start at zero to prevent client player desync
	args = append(args, "-avoid_negative_ts", "make_zero")
	// -max_muxing_queue_size 9999: Expand the muxing queue limit to prevent packet drop errors during startup spikes
	args = append(args, "-max_muxing_queue_size", "9999")
	// -f mpegts -: Output in MPEG Transport Stream (MPEG-TS) format piped directly to standard output (stdout)
	args = append(args, "-f", "mpegts")
	args = append(args, "-mpegts_pmt_start_pid", "4096")
	args = append(args, "-mpegts_start_pid", "256")
	args = append(args, "-")
	return args
}

func writeDiscontinuityPacket(w io.Writer, pid uint16) {
	pkt := make([]byte, 188)
	pkt[0] = 0x47          // Sync byte
	pkt[1] = byte(pid >> 8) // High bits of PID
	pkt[2] = byte(pid & 0xFF) // Low bits of PID
	pkt[3] = 0x20          // Adaptation field only, no payload
	pkt[4] = 0x01          // Adaptation field length = 1
	pkt[5] = 0x80          // Discontinuity indicator = 1
	for i := 6; i < 188; i++ {
		pkt[i] = 0xFF // Padding
	}
	_, _ = w.Write(pkt)
}
