package epg

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Source struct {
	url    string
	client *http.Client
	cache  []byte
	mu     sync.RWMutex
	last   time.Time
}

func New(url string, client *http.Client) *Source {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Source{url: url, client: client}
}

func (s *Source) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	data := s.cache
	s.mu.RUnlock()
	if len(data) == 0 {
		http.Error(w, "EPG not available", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(data)
}

func (s *Source) Refresh() error {
	req, err := http.NewRequest("GET", s.url, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("epg fetch status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cache = data
	s.last = time.Now()
	s.mu.Unlock()
	return nil
}
