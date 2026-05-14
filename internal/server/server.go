package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aymerici/plexishow/internal/epg"
	"github.com/aymerici/plexishow/internal/hdhr"
	"github.com/aymerici/plexishow/internal/metrics"
	"github.com/aymerici/plexishow/internal/store"
	"github.com/aymerici/plexishow/internal/stream"
)

type Server struct {
	baseURL  string
	store    *store.Store
	streamer *stream.Manager
	epg      *epg.Source
	hdhr     *hdhr.Handler
	metrics  *metrics.Registry
}

func New(baseURL string, st *store.Store, streamer *stream.Manager, epg *epg.Source, metrics *metrics.Registry) *Server {
	return &Server{
		baseURL:  baseURL,
		store:    st,
		streamer: streamer,
		epg:      epg,
		hdhr:     hdhr.NewHandler(baseURL, st),
		metrics:  metrics,
	}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/device.xml", s.hdhr.DeviceXML)
	mux.HandleFunc("/discover.json", s.hdhr.Discover)
	mux.HandleFunc("/lineup.json", s.hdhr.Lineup)
	mux.HandleFunc("/lineup_status.json", s.hdhr.LineupStatus)
	mux.HandleFunc("/channels.m3u", s.ServeM3U)
	if s.epg != nil {
		mux.HandleFunc("/epg.xml", s.epg.ServeHTTP)
	}
	mux.HandleFunc("/stream/", s.streamer.ServeHTTP)
	mux.HandleFunc("/metrics", s.metrics.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Router(),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	fmt.Printf("Listening on %s\n", addr)
	return srv.ListenAndServe()
}
