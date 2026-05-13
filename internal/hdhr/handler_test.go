package hdhr

import (
	"encoding/json"
	"encoding/xml"
	"net/http/httptest"
	"testing"

	"github.com/aymerici/plexishow/internal/m3u"
	"github.com/aymerici/plexishow/internal/store"
)

func TestDiscover(t *testing.T) {
	st := store.New()
	h := NewHandler("http://localhost:8080", st)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/discover.json", nil)
	h.Discover(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var d Discover
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.TunerCount != 4 {
		t.Errorf("tunercount = %d", d.TunerCount)
	}
}

func TestLineup(t *testing.T) {
	st := store.New()
	st.Replace([]m3u.Channel{
		{ID: "ch1", Name: "One", TVGID: "1.1"},
	})
	h := NewHandler("http://localhost:8080", st)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/lineup.json", nil)
	h.Lineup(w, r)
	var items []LineupItem
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].GuideName != "One" {
		t.Errorf("unexpected lineup: %+v", items)
	}
}

func TestDeviceXML(t *testing.T) {
	st := store.New()
	h := NewHandler("http://localhost:8080", st)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/device.xml", nil)
	h.DeviceXML(w, r)
	var d DeviceDescription
	if err := xml.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.FriendlyName == "" {
		t.Error("empty friendly name")
	}
}
