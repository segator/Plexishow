package server

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) ServeM3U(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, ch := range s.store.All() {
		fmt.Fprintf(&b, "#EXTINF:-1 tvg-id=%q tvg-logo=%q group-title=%q,%s\n",
			ch.TVGID, ch.TVGLogo, ch.Group, ch.Name)
		fmt.Fprintf(&b, "%s/stream/%s\n", s.baseURL, ch.ID)
	}
	_, _ = w.Write([]byte(b.String()))
}
