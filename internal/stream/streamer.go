package stream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/aymerici/plexishow/internal/config"
	"github.com/aymerici/plexishow/internal/m3u"
	"github.com/aymerici/plexishow/internal/metrics"
	"github.com/aymerici/plexishow/internal/store"
)

type Manager struct {
	cfg     config.Config
	store   *store.Store
	metrics *metrics.Registry
	mu      sync.Mutex
	active  map[string]*exec.Cmd
	sem     chan struct{}
}

func NewManager(cfg config.Config, st *store.Store, metrics *metrics.Registry) *Manager {
	return &Manager{
		cfg:     cfg,
		store:   st,
		metrics: metrics,
		active:  make(map[string]*exec.Cmd),
		sem:     make(chan struct{}, cfg.MaxStreams),
	}
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/stream/")
	ch, ok := m.store.Get(id)
	if !ok {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}

	select {
	case m.sem <- struct{}{}:
	default:
		http.Error(w, "max concurrent streams reached", http.StatusServiceUnavailable)
		return
	}
	defer func() { <-m.sem }()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	args := buildArgs(ch)
	fmt.Printf("[stream] %s: %s %s\n", ch.Name, m.cfg.FFmpegPath, strings.Join(args, " "))
	//#nosec G204 -- ffmpeg path from config, intentional
	cmd := exec.CommandContext(ctx, m.cfg.FFmpegPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.metrics.IncErrors()
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	stderr, _ := cmd.StderrPipe()
	defer func() { _ = stdout.Close() }()
	if stderr != nil {
		go func() { _, _ = io.Copy(io.Discard, stderr) }()
	}

	if err := cmd.Start(); err != nil {
		m.metrics.IncErrors()
		http.Error(w, "failed to start stream", http.StatusInternalServerError)
		return
	}
	m.metrics.IncActive()
	defer m.metrics.DecActive()
	m.track(ch.ID, cmd)
	defer m.untrack(ch.ID)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	w.Header().Set("Content-Type", "video/mp2t")
	w.WriteHeader(http.StatusOK)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	timer := time.NewTimer(m.cfg.StreamTimeout)
	defer timer.Stop()
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(w, stdout)
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (m *Manager) track(id string, cmd *exec.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active[id] = cmd
}

func (m *Manager) untrack(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, id)
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cmd := range m.active {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

func buildArgs(ch m3u.Channel) []string {
	var args []string

	args = append(args, "-y")

	if ch.KeyID != "" && ch.Key != "" {
		args = append(args, "-cenc_decryption_key", fmt.Sprintf("%s:%s", ch.KeyID, ch.Key))
	}

	// ffmpeg has a dedicated -user_agent flag; everything else goes via -headers
	if ua, ok := ch.Headers["User-Agent"]; ok {
		args = append(args, "-user_agent", ua)
	}
	for k, v := range ch.Headers {
		if k == "User-Agent" {
			continue
		}
		args = append(args, "-headers", fmt.Sprintf("%s: %s", k, v))
	}

	args = append(args, "-re", "-i", ch.URL)
	args = append(args, "-c:v", "copy", "-c:a", "aac", "-f", "mpegts", "-")
	return args
}
