package store

import (
	"testing"

	"github.com/aymerici/plexishow/internal/m3u"
)

func TestReplaceAndGet(t *testing.T) {
	s := New()
	s.Replace([]m3u.Channel{
		{ID: "foo", Name: "Foo"},
		{ID: "bar", Name: "Bar"},
	})
	c, ok := s.Get("foo")
	if !ok || c.Name != "Foo" {
		t.Errorf("expected Foo")
	}
	all := s.All()
	if len(all) != 2 {
		t.Errorf("expected 2 channels")
	}
}

func TestGetMissing(t *testing.T) {
	s := New()
	_, ok := s.Get("nope")
	if ok {
		t.Error("expected false")
	}
}
