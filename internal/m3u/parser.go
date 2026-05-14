package m3u

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	extinfRe   = regexp.MustCompile(`#EXTINF:-?\d+\s+(.*),(.*)`)
	attrRe     = regexp.MustCompile(`(\S+)="([^"]+)"`)
	kodipropRe = regexp.MustCompile(`#KODIPROP:inputstream\.adaptive\.license_key=(?:\{)?([a-fA-F0-9]+):([a-fA-F0-9]+)(?:\})?`)
	vlcoptRe   = regexp.MustCompile(`#EXTVLCOPT:(.+)`)
)

func Parse(data []byte) ([]Channel, string, error) {
	if !bytes.HasPrefix(data, []byte("#EXTM3U")) {
		return nil, "", fmt.Errorf("missing #EXTM3U header")
	}
	var channels []Channel
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var current *Channel
	var epgURL string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#EXTM3U") {
			if epgURL == "" && strings.HasPrefix(line, "#EXTM3U") {
				epgURL = extractEPGURL(line)
			}
			continue
		}

		if strings.HasPrefix(line, "#EXTINF:") {
			if current != nil {
				channels = append(channels, *current)
			}
			current = &Channel{Headers: make(map[string]string)}
			m := extinfRe.FindStringSubmatch(line)
			if len(m) == 3 {
				attrs := m[1]
				current.Name = strings.TrimSpace(m[2])
				for _, am := range attrRe.FindAllStringSubmatch(attrs, -1) {
					if len(am) == 3 {
						switch am[1] {
						case "tvg-id":
							current.TVGID = am[2]
						case "tvg-logo":
							current.TVGLogo = am[2]
						case "group-title":
							current.Group = am[2]
						}
					}
				}
			}
			continue
		}

		if strings.HasPrefix(line, "#KODIPROP:") {
			m := kodipropRe.FindStringSubmatch(line)
			if len(m) == 3 && current != nil {
				current.KeyID = strings.ToLower(m[1])
				current.Key = strings.ToLower(m[2])
			}
			// Parse stream_headers (e.g. X-TCDN-token)
			if strings.Contains(line, "stream_headers=") && current != nil {
				parseStreamHeaders(current, line)
			}
			continue
		}

		if strings.HasPrefix(line, "#EXTVLCOPT:") {
			m := vlcoptRe.FindStringSubmatch(line)
			if len(m) == 2 && current != nil {
				parseVLCOpt(current, m[1])
			}
			continue
		}

		if current != nil && !strings.HasPrefix(line, "#") {
			current.URL = line
			if current.TVGID != "" {
				current.ID = sanitizeID(current.TVGID)
			} else {
				u, _ := url.Parse(line)
				if u != nil {
					current.ID = sanitizeID(u.Path)
				} else {
					current.ID = sanitizeID(line)
				}
			}
			channels = append(channels, *current)
			current = nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, epgURL, err
	}
	return channels, epgURL, nil
}

// extractEPGURL parses the url-tvg attribute from the #EXTM3U line.
func extractEPGURL(line string) string {
	m := regexp.MustCompile(`url-tvg="([^"]+)"`).FindStringSubmatch(line)
	if len(m) == 2 {
		urls := strings.Split(m[1], ",")
		return strings.TrimSpace(urls[0])
	}
	return ""
}

func parseVLCOpt(ch *Channel, opt string) {
	if strings.HasPrefix(opt, "http-referrer=") {
		ch.Headers["Referer"] = strings.TrimPrefix(opt, "http-referrer=")
	} else if strings.HasPrefix(opt, "http-user-agent=") {
		ch.Headers["User-Agent"] = strings.TrimPrefix(opt, "http-user-agent=")
	} else if strings.HasPrefix(opt, "http-header=") {
		rest := strings.TrimPrefix(opt, "http-header=")
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) == 2 {
			ch.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
}

// parseStreamHeaders extracts key=value pairs from
// #KODIPROP:inputstream.adaptive.stream_headers=Key=Value
func parseStreamHeaders(ch *Channel, line string) {
	prefix := "#KODIPROP:inputstream.adaptive.stream_headers="
	idx := strings.Index(line, prefix)
	if idx == -1 {
		return
	}
	rest := line[idx+len(prefix):]
	// Some playlists put multiple headers separated by \r\n
	for _, h := range strings.Split(rest, "\r\n") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		parts := strings.SplitN(h, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			if strings.EqualFold(key, "x-tcdn-token") {
				key = "X-TCDN-token"
			}
			ch.Headers[key] = strings.TrimSpace(parts[1])
		}
	}
}

func sanitizeID(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
