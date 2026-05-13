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

	args := buildArgs(m.cfg, ch)
	cmd := exec.CommandContext(ctx, m.cfg.FFmpegPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.metrics.IncErrors()
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	stderr, _ := cmd.StderrPipe()
	defer stdout.Close()
	if stderr != nil {
		go io.Copy(io.Discard, stderr)
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

func buildArgs(cfg config.Config, ch m3u.Channel) []string {
	args := []string{
		"-fflags", "+discardcorrupt",
		"-headers", buildHeaders(ch.Headers),
		"-i", ch.URL,
		"-c:v", "copy",
		"-c:a", "aac",
		"-f", "mpegts",
		"-",
	}
	if ch.KeyID != "" && ch.Key != "" {
		newArgs := make([]string, 0, len(args)+4)
		newArgs = append(newArgs, args[0:2]...)
		newArgs = append(newArgs, "-cenc_decryption_key", fmt.Sprintf("%s:%s", ch.KeyID, ch.Key))
		newArgs = append(newArgs, args[2:]...)
		args = newArgs
	}
	return args
}

func buildHeaders(h map[string]string) string {
	var b strings.Builder
	for k, v := range h {
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	return b.String()
}
