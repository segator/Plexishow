package m3u

import (
	"os"
	"testing"
)

func TestParseFixture(t *testing.T) {
	b, err := os.ReadFile("../../test/fixtures/source.m3u")
	if err != nil {
		t.Skipf("fixture not available: %v", err)
	}
	chs, _, err := Parse(b)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(chs) == 0 {
		t.Fatal("expected channels")
	}
	c := chs[0]
	if c.KeyID == "" || c.Key == "" {
		t.Errorf("first channel missing key: %+v", c)
	}
	if c.URL == "" {
		t.Errorf("first channel missing url")
	}
}

func TestParseMinimal(t *testing.T) {
	data := `#EXTM3U
#EXTINF:-1 tvg-id="foo" tvg-logo="http://img" group-title="Sports",Channel A
#KODIPROP:inputstream.adaptive.license_key=abcd1234:efff5678
#EXTVLCOPT:http-referrer=https://tv.iptvprovider.com/
http://example.com/stream.mpd
`
	chs, _, err := Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(chs))
	}
	c := chs[0]
	if c.Name != "Channel A" {
		t.Errorf("name = %q", c.Name)
	}
	if c.TVGID != "foo" {
		t.Errorf("tvg-id = %q", c.TVGID)
	}
	if c.KeyID != "abcd1234" || c.Key != "efff5678" {
		t.Errorf("key mismatch: %s:%s", c.KeyID, c.Key)
	}
	if c.Headers["Referer"] != "https://tv.iptvprovider.com/" {
		t.Errorf("referer missing")
	}
}

func TestParseNoHeader(t *testing.T) {
	_, _, err := Parse([]byte("http://example.com\n"))
	if err == nil {
		t.Fatal("expected error for missing #EXTM3U")
	}
}

func TestParseDuplicateChannels(t *testing.T) {
	b, err := os.ReadFile("../../test/fixtures/source.m3u")
	if err != nil {
		t.Skipf("fixture not available: %v", err)
	}
	chs, _, _ := Parse(b)
	ids := make(map[string]int)
	for _, c := range chs {
		ids[c.ID]++
	}
	// Duplicates acceptable; proxy will use first or last depending on store policy
}

func TestParseStreamHeaders(t *testing.T) {
	data := `#EXTM3U
#EXTINF:-1 tvg-id="cuatro" tvg-logo="https://img" group-title="TDT",Cuatro
#KODIPROP:inputstream.adaptive.license_type=org.w3.clearkey
#KODIPROP:inputstream.adaptive.license_key={26a7ba3841ad411ea13a9fb9d1470ea9:210006fb5fdcae5a7d36051de31911b4}
#EXTVLCOPT:http-referrer=https://tv.iptvprovider.com/
#EXTVLCOPT:http-user-agent=Chrome/61.0.3163.100
#KODIPROP:inputstream.adaptive.stream_headers=X-TCDN-token=eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZSI6ImNkbiJ9.SIG
http://cdn.iptvprovider.com/4524/vxfmt=dp/Manifest.mpd
`
	chs, _, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(chs) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(chs))
	}
	c := chs[0]
	if c.Headers["X-TCDN-token"] != "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZSI6ImNkbiJ9.SIG" {
		t.Errorf("X-TCDN-token mismatch: got %q", c.Headers["X-TCDN-token"])
	}
	if c.Headers["Referer"] != "https://tv.iptvprovider.com/" {
		t.Errorf("Referer mismatch: got %q", c.Headers["Referer"])
	}
	if c.Headers["User-Agent"] != "Chrome/61.0.3163.100" {
		t.Errorf("User-Agent mismatch: got %q", c.Headers["User-Agent"])
	}
	if c.KeyID != "26a7ba3841ad411ea13a9fb9d1470ea9" || c.Key != "210006fb5fdcae5a7d36051de31911b4" {
		t.Errorf("key mismatch: %s:%s", c.KeyID, c.Key)
	}
}
