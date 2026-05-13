package store

import (
	"sync"

	"github.com/aymerici/plexishow/internal/m3u"
)

type Store struct {
	mu   sync.RWMutex
	data map[string]m3u.Channel
	list []m3u.Channel
}

func New() *Store {
	return &Store{data: make(map[string]m3u.Channel)}
}

func (s *Store) Replace(chs []m3u.Channel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = make([]m3u.Channel, len(chs))
	copy(s.list, chs)
	s.data = make(map[string]m3u.Channel, len(chs))
	for _, c := range chs {
		s.data[c.ID] = c
	}
}

func (s *Store) Get(id string) (m3u.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.data[id]
	return c, ok
}

func (s *Store) All() []m3u.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]m3u.Channel, len(s.list))
	copy(out, s.list)
	return out
}
