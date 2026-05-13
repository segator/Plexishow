package main

import "testing"

func TestVersionSet(t *testing.T) {
	if version == "" {
		t.Error("version should not be empty")
	}
}

func TestDefaultBaseURL(t *testing.T) {
	tests := []struct {
		listenAddr string
		want       string
	}{
		{":8080", "http://localhost:8080"},
		{"127.0.0.1:9090", "http://127.0.0.1:9090"},
		{"localhost:7070", "http://localhost:7070"},
		{"invalid-listen-addr", "http://localhostinvalid-listen-addr"},
	}
	for _, tt := range tests {
		got := defaultBaseURL(tt.listenAddr)
		if got != tt.want {
			t.Errorf("defaultBaseURL(%q) = %q, want %q", tt.listenAddr, got, tt.want)
		}
	}
}
