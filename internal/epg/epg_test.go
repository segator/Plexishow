package epg

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchAndServe(t *testing.T) {
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><tv><channel id="foo"></channel></tv>`))
	}))
	defer src.Close()

	e := New(src.URL, &http.Client{})
	if err := e.Refresh(); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/epg.xml", nil)
	e.ServeHTTP(w, r)
	body, _ := io.ReadAll(w.Result().Body)
	if !strings.Contains(string(body), `<channel id="foo">`) {
		t.Errorf("unexpected body: %s", body)
	}
}
