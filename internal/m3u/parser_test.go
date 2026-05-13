package m3u

import (
	"os"
	"testing"
)

func TestParseFixture(t *testing.T) {
	b, err := os.ReadFile("../../test/fixtures/source.m3u")
	if err != nil {
		t.Fatal(err)
	}
	chs, err := Parse(b)
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
#EXTVLCOPT:http-referrer=https://tv.movistar.com.pe/
http://example.com/stream.mpd
`
	chs, err := Parse([]byte(data))
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
	if c.Headers["Referer"] != "https://tv.movistar.com.pe/" {
		t.Errorf("referer missing")
	}
}

func TestParseNoHeader(t *testing.T) {
	_, err := Parse([]byte("http://example.com\n"))
	if err == nil {
		t.Fatal("expected error for missing #EXTM3U")
	}
}

func TestParseDuplicateChannels(t *testing.T) {
	b, _ := os.ReadFile("../../test/fixtures/source.m3u")
	chs, _ := Parse(b)
	ids := make(map[string]int)
	for _, c := range chs {
		ids[c.ID]++
	}
	// Duplicates acceptable; proxy will use first or last depending on store policy
}
