# Plexishow Backlog & Future Explorations 🚀

This document acts as a persistent repository backlog of ideas, feature proposals, and exploratory tasks to transition Plexishow into a 10/10 Enterprise-grade IPTV/Plex DVR companion.

---

## 📅 Roadmap & Exploratory Tasks

### 1. ⚡ EPG Pre-fetching & Pre-Tuning
* **Goal**: Drop the Plex "Tune Time" (channel zapping latency) to **under 500ms**.
* **Concept**:
  * Implement an active sliding-window pre-tuner.
  * When a client tunes to channel $N$, or starts scrolling through the EPG on Plex, Plexishow can pre-initialize the next/previous channels ($N-1$ and $N+1$) in a low-priority background buffer.
  * The first 3-4 segments of these streams are downloaded, decrypted, and cached in a RAM-backed circular cache.
  * When the user presses "Channel Up/Down", the stream starts playing instantly.

### 2. 📺 HTML5 Web UI & Google Chromecast Casting
* **Goal**: Provide a standalone premium web experience to browse and cast channels if Plex DVR integration has limitations.
* **Concept**:
  * Build a beautiful glassmorphism dark-mode Single Page Web UI served directly by Go's `embed` package.
  * Embed an HTML5 `HLS` / `dash.js` / `video.js` video player.
  * Integrate the **Google Cast SDK (Chromecast)** to support casting the decrypted MPEG-TS/HLS streams straight from your computer/phone browser to any Chromecast-enabled television on the LAN.

### 3. 🐍 M3U Generation Script Porting & Integration
* **Goal**: Make Plexishow a single-binary solution containing both generation and streaming.
* **Concept**:
  * Port or integrate Isaac's custom Python M3U generation scripts.
  * Can be added either as:
    * A CLI subcommand: `plexishow generate-m3u --config config.yaml`
    * Or a background worker that runs periodically, generating a fresh local M3U that Plexishow immediately hot-reloads.

### 4. 🔄 Dynamic GPU-to-CPU Failover (Transcode Robustness)
* **Goal**: Prevent black screens or dropped Plex streams when hardware graphics drivers fail or hit VRAM limits.
* **Concept**:
  * Track hardware transcode health during process initialization.
  * If a fast hardware encoder command (e.g. `h264_nvenc` or `h264_vaapi`) exits abruptly during startup with a driver/VRAM error, instantly catch the error and respawn `ffmpeg` using CPU fallback (`libx264` with `ultrafast` speed) or raw direct copy.

### 5. 🔑 Automated ClearKey Provider / License Resolver
* **Goal**: Avoid manual M3U editing when IPTV providers rotate or update ClearKey encryption keys.
* **Concept**:
  * Define an external `key_provider_url` config field.
  * If a channel's key decays or changes, Plexishow queries the external API in the background using the `KeyID` (KID) to fetch, validate, and hot-reload the fresh decryption keys.

### 6. 🕒 Server-Side Timeshift Buffer
* **Goal**: Support long pauses and rewinds in Plex Live TV without connection drops or network timeouts.
* **Concept**:
  * Implement an optional file-backed or RAM-backed circular cache in a temporary directory (e.g., `/tmp` or a mounted `tmpfs` RAM-disk).
  * As the stream is decrypted and downloaded, keep the last 15-30 minutes of MPEG-TS data in memory/disk.
  * When a Plex client pauses, keep the upstream HTTP connection and `ffmpeg` transcoding process alive in the background (writing to the circular buffer).
  * If Plex resumes, serve the stream from the cached buffer. If Plex experiences network congestion, the server acts as an elastic buffer, padding or throttling the output dynamically without severing the TCP pipe.

### 8. ⚡ Interceptor & Stripper de Manifiesto XML DASH (`suggestedPresentationDelay`)
* **Goal**: Forzar a `ffmpeg` a sintonizar en el borde absoluto del directo, eliminando los 20 segundos de retraso por defecto (esencial para fútbol y deportes en vivo).
* **Concept**:
  * **El problema de latencia**: El proveedor de IPTV (Movistar+) añade a propósito una directiva XML en su manifiesto `.mpd` llamada `suggestedPresentationDelay="PT20S"`. Esto obliga por estándar a cualquier cliente (como `ffmpeg`) a sintonizar 20 segundos por detrás del directo real para dar margen a la red.
  * **La solución**: Implementar un proxy/manejador HTTP interno en Plexishow. En lugar de pasarle a `ffmpeg` la URL directa del manifiesto remoto del proveedor, Plexishow descargará el manifiesto en memoria, parseará el XML y **eliminará o reescribirá** la etiqueta `suggestedPresentationDelay` poniéndola a `PT0S` (0 segundos) o eliminándola por completo.
  * **El resultado**: Plexishow le pasará a `ffmpeg` la URL local de este manifiesto "limpio". Al no tener el retraso sugerido, `ffmpeg` descargará inmediatamente el segmento más nuevo del directo real en el estadio, reduciendo la latencia de emisión a prácticamente **0 segundos** de lag de origen en el reproductor del usuario.

---

## 🛠️ Explored & Fully Implemented Features

* [x] **Instant Channel Loading Video Placeholder**: High-end zapping experience offering a **0-second subjective tune time**. Serves a beautifully pre-rendered 10-second Full HD (1080p) Stereo 48kHz retro film countdown loop in RAM instantly on client connection. Handled in Go via thread-safe hot-swapping, custom discontinuity indicator packets (PID 0x0100 for H.264, PID 0x0101 for AAC), and automatic channel draining to prevent network stuttering.
* [x] **Stream Multiplexing (Fan-Out / Sharing)**: The streaming core is 100% shared! Tuning 2 clients to the same channel does **not** launch two `ffmpeg` transcode processes; the underlying thread-safe subscriber architecture redirects the decrypted stdout packet buffer to all active clients seamlessly.
* [x] **Dynamic GPU Acceleration**: Fully configurable support for NVENC, VAAPI, and QSV hardware pipelines.
* [x] **Network Recovery (HTTP Reconnects)**: Micro-second network drop safety via custom configurable `-reconnect 1 -rw_timeout` parameters.
* [x] **Advanced Prometheus Metrics**: Custom light-weight endpoints serving real-time viewer distribution per channel, network bytes sent, and playlist statistics.
