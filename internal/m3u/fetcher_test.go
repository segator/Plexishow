package m3u

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aymerici/plexishow/internal/config"
	"github.com/aymerici/plexishow/internal/metrics"
)

type fakeStore struct {
	channels []Channel
}

func (f *fakeStore) Replace(chs []Channel) {
	f.channels = make([]Channel, len(chs))
	copy(f.channels, chs)
}

func (f *fakeStore) All() []Channel {
	out := make([]Channel, len(f.channels))
	copy(out, f.channels)
	return out
}

func TestFetcherPull(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TCDN-token") != "abc" {
			t.Errorf("missing token header")
		}
		_, _ = w.Write([]byte(`#EXTM3U
#EXTINF:-1 tvg-id="x",Test
#KODIPROP:inputstream.adaptive.license_key=a:b
http://s/stream.mpd
`))
	}))
	defer srv.Close()

	st := &fakeStore{}
	f := NewFetcher(&config.Config{
		M3UURL:         srv.URL,
		DefaultHeaders: config.Headers{Token: "abc", Referer: "ref", UserAgent: "ua"},
		StreamTimeout:  5 * time.Second,
	}, st)

	metricsReg := metrics.New()
	f.SetMetrics(metricsReg)

	if err := f.Pull(); err != nil {
		t.Fatal(err)
	}
	chs := st.All()
	if len(chs) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(chs))
	}
	if chs[0].Headers["X-TCDN-token"] != "abc" {
		t.Errorf("token not merged")
	}
	if chs[0].Headers["User-Agent"] != "ua" {
		t.Errorf("ua not merged")
	}
}
