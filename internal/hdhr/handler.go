package hdhr

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"

	"github.com/aymerici/plexishow/internal/store"
)

type Handler struct {
	baseURL string
	store   *store.Store
}

func NewHandler(baseURL string, s *store.Store) *Handler {
	return &Handler{baseURL: baseURL, store: s}
}

func (h *Handler) DeviceXML(w http.ResponseWriter, r *http.Request) {
	d := DeviceDescription{
		SpecMajor:    1,
		SpecMinor:    0,
		URLBase:      h.baseURL,
		DeviceType:   "urn:schemas-upnp-org:device:MediaServer:1",
		FriendlyName: "Plexishow",
		Manufacturer: "Silicondust",
		ModelNumber:  "HDHR3-US",
		ModelName:    "HDHomeRun",
		SerialNumber: "12345678",
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(d)
}

func (h *Handler) Discover(w http.ResponseWriter, r *http.Request) {
	d := Discover{
		FriendlyName:    "Plexishow",
		Manufacturer:    "Silicondust",
		ModelNumber:     "HDHR3-US",
		FirmwareName:    "hdhomerun3_atsc",
		TunerCount:      4,
		FirmwareVersion: "20240101",
		DeviceID:        "12345678",
		DeviceAuth:      "testauth",
		BaseURL:         h.baseURL,
		LineupURL:       h.baseURL + "/lineup.json",
	}
	writeJSON(w, d)
}

func (h *Handler) LineupStatus(w http.ResponseWriter, r *http.Request) {
	s := LineupStatus{
		ScanInProgress: 0,
		ScanPossible:   1,
		Source:         "Cable",
		SourceList:     []string{"Antenna", "Cable"},
	}
	writeJSON(w, s)
}

func (h *Handler) Lineup(w http.ResponseWriter, r *http.Request) {
	channels := h.store.All()
	items := make([]LineupItem, len(channels))
	for i, ch := range channels {
		gn := ch.TVGID
		if gn == "" {
			gn = ch.ID
		}
		items[i] = LineupItem{
			GuideNumber: gn,
			GuideName:   ch.Name,
			URL:         fmt.Sprintf("%s/stream/%s", h.baseURL, ch.ID),
			HD:          1,
		}
	}
	writeJSON(w, items)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.MarshalIndent(v, "", "  ")
	w.Write(b)
}
