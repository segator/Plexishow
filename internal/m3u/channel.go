package m3u

// Channel represents a parsed IPTV channel with decryption metadata.
type Channel struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	TVGID   string            `json:"tvg_id,omitempty"`
	TVGLogo string            `json:"tvg_logo,omitempty"`
	Group   string            `json:"group_title,omitempty"`
	URL     string            `json:"url"`
	KeyID   string            `json:"key_id,omitempty"`  // hex
	Key     string            `json:"key,omitempty"`     // hex
	Headers map[string]string `json:"headers,omitempty"` // merged defaults + per-channel
}
