package stream

import (
	"bufio"
	"context"
	"fmt"
	"hash/fnv"
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
	subs        map[chan []byte]struct{}
	done        chan struct{}
	idleTimer   *time.Timer
	idleTimeout time.Duration
}

func (s *session) addSub(ch chan []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subs == nil {
		return false
	}
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
	s.subs[ch] = struct{}{}
	return true
}

func (s *session) removeSub(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subs == nil {
		return
	}
	delete(s.subs, ch)
	close(ch)
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

func (s *session) broadcast(stderr io.ReadCloser, name string) {
	defer close(s.done)
	if stderr != nil {
		go func() {
			color := channelColor(name)
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				fmt.Printf("%s[%s]%s %s\n", color, name, ansiReset, scanner.Text())
			}
		}()
	}
	buf := make([]byte, 32*1024)
	for {
		n, err := s.stdout.Read(buf)
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			s.mu.Lock()
			for ch := range s.subs {
				select {
				case ch <- data:
				default:
					close(ch)
					delete(s.subs, ch)
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
	cfg      config.Config
	store    *store.Store
	metrics  *metrics.Registry
	mu       sync.Mutex
	sessions map[string]*session
	sem      chan struct{}
}

func NewManager(cfg config.Config, st *store.Store, metrics *metrics.Registry) *Manager {
	return &Manager{
		cfg:      cfg,
		store:    st,
		metrics:  metrics,
		sessions: make(map[string]*session),
		sem:      make(chan struct{}, cfg.MaxStreams),
	}
}

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/stream/")
	ch, ok := m.store.Get(id)
	if !ok {
		http.Error(w, "channel not found", http.StatusNotFound)
		return
	}

	chData := make(chan []byte, 50)
	defer close(chData)

	m.mu.Lock()
	sess, exists := m.sessions[id]
	m.mu.Unlock()

	if !exists || !sess.addSub(chData) {
		if exists {
			// session died between the unlock and addSub
		}
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
		sess.addSub(chData)
	}
	defer sess.removeSub(chData)

	w.Header().Set("Content-Type", "video/mp2t")
	w.WriteHeader(http.StatusOK)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	timer := time.NewTimer(m.cfg.StreamTimeout)
	defer timer.Stop()

	for {
		select {
		case data, ok := <-chData:
			if !ok {
				return
			}
			if _, err := w.Write(data); err != nil {
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

	args := buildArgs(ch)
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
		subs:        make(map[chan []byte]struct{}),
		done:        make(chan struct{}),
		idleTimeout: defaultIdleTimeout,
	}

	m.mu.Lock()
	m.sessions[ch.ID] = sess
	m.mu.Unlock()

	go func() {
		sess.broadcast(stderr, ch.Name)
		m.mu.Lock()
		delete(m.sessions, ch.ID)
		m.mu.Unlock()
		<-m.sem
		m.metrics.DecActive()
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

func buildArgs(ch m3u.Channel) []string {
	var args []string

	args = append(args, "-y")

	if ch.Key != "" {
		args = append(args, "-cenc_decryption_key", ch.Key)
	}

	// ffmpeg has a dedicated -user_agent flag; everything else goes via -headers
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
		args = append(args, "-headers", hdr.String())
	}

	args = append(args, "-re", "-i", ch.URL)
	args = append(args, "-c:v", "copy", "-c:a", "aac", "-f", "mpegts", "-")
	return args
}
