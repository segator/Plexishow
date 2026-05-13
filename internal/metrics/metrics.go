package metrics

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
)

type Registry struct {
	activeStreams  int64
	streamErrors   int64
	m3uChannels    int64
	lastRefresh    int64
	lastEpgRefresh int64

	mu        sync.RWMutex
	viewers   map[string]int64
	bytesSent map[string]uint64
}

func New() *Registry {
	return &Registry{
		viewers:   make(map[string]int64),
		bytesSent: make(map[string]uint64),
	}
}

func (r *Registry) IncActive() { atomic.AddInt64(&r.activeStreams, 1) }
func (r *Registry) DecActive() { atomic.AddInt64(&r.activeStreams, -1) }
func (r *Registry) IncErrors() { atomic.AddInt64(&r.streamErrors, 1) }

func (r *Registry) SetM3UChannels(count int) {
	atomic.StoreInt64(&r.m3uChannels, int64(count))
}

func (r *Registry) SetLastRefresh(timestamp int64) {
	atomic.StoreInt64(&r.lastRefresh, timestamp)
}

func (r *Registry) SetEPGLastRefresh(timestamp int64) {
	atomic.StoreInt64(&r.lastEpgRefresh, timestamp)
}

func (r *Registry) AddViewer(channel string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.viewers[channel]++
}

func (r *Registry) RemoveViewer(channel string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.viewers[channel] > 0 {
		r.viewers[channel]--
	}
}

func (r *Registry) AddBytesSent(channel string, bytes int) {
	if bytes <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	//#nosec G115 -- safe conversion after bounds check
	r.bytesSent[channel] += uint64(bytes)
}

func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)

		f := func(format string, args ...any) {
			_, _ = fmt.Fprintf(w, format, args...)
		}

		// Custom app metrics
		f("# HELP plexishow_active_streams Current active ffmpeg streams\n")
		f("# TYPE plexishow_active_streams gauge\n")
		f("plexishow_active_streams %d\n", atomic.LoadInt64(&r.activeStreams))

		f("# HELP plexishow_stream_errors_total Total stream errors\n")
		f("# TYPE plexishow_stream_errors_total counter\n")
		f("plexishow_stream_errors_total %d\n", atomic.LoadInt64(&r.streamErrors))

		f("# HELP plexishow_m3u_channels_total Total channels loaded from M3U\n")
		f("# TYPE plexishow_m3u_channels_total gauge\n")
		f("plexishow_m3u_channels_total %d\n", atomic.LoadInt64(&r.m3uChannels))

		f("# HELP plexishow_m3u_last_refresh_timestamp_seconds Last successful M3U refresh timestamp\n")
		f("# TYPE plexishow_m3u_last_refresh_timestamp_seconds gauge\n")
		f("plexishow_m3u_last_refresh_timestamp_seconds %d\n", atomic.LoadInt64(&r.lastRefresh))

		f("# HELP plexishow_epg_last_refresh_timestamp_seconds Last successful EPG refresh timestamp\n")
		f("# TYPE plexishow_epg_last_refresh_timestamp_seconds gauge\n")
		f("plexishow_epg_last_refresh_timestamp_seconds %d\n", atomic.LoadInt64(&r.lastEpgRefresh))

		// Channel viewers and bytes sent
		r.mu.RLock()
		if len(r.viewers) > 0 {
			f("# HELP plexishow_channel_viewers Current viewers per channel\n")
			f("# TYPE plexishow_channel_viewers gauge\n")
			for ch, count := range r.viewers {
				f("plexishow_channel_viewers{channel=%q} %d\n", ch, count)
			}
		}
		if len(r.bytesSent) > 0 {
			f("# HELP plexishow_channel_bytes_sent_total Total bytes sent per channel\n")
			f("# TYPE plexishow_channel_bytes_sent_total counter\n")
			for ch, bytes := range r.bytesSent {
				f("plexishow_channel_bytes_sent_total{channel=%q} %d\n", ch, bytes)
			}
		}
		r.mu.RUnlock()

		// Go runtime metrics
		f("# HELP go_goroutines Number of goroutines\n")
		f("# TYPE go_goroutines gauge\n")
		f("go_goroutines %d\n", runtime.NumGoroutine())

		f("# HELP go_memstats_alloc_bytes_bytes Number of bytes allocated and still in use\n")
		f("# TYPE go_memstats_alloc_bytes_bytes gauge\n")
		f("go_memstats_alloc_bytes_bytes %d\n", ms.Alloc)

		f("# HELP go_memstats_sys_bytes Total bytes obtained from system\n")
		f("# TYPE go_memstats_sys_bytes gauge\n")
		f("go_memstats_sys_bytes %d\n", ms.Sys)

		f("# HELP go_memstats_heap_alloc_bytes Heap allocation bytes\n")
		f("# TYPE go_memstats_heap_alloc_bytes gauge\n")
		f("go_memstats_heap_alloc_bytes %d\n", ms.HeapAlloc)

		f("# HELP go_memstats_heap_inuse_bytes Heap in-use bytes\n")
		f("# TYPE go_memstats_heap_inuse_bytes gauge\n")
		f("go_memstats_heap_inuse_bytes %d\n", ms.HeapInuse)

		f("# HELP go_memstats_heap_objects Number of allocated heap objects\n")
		f("# TYPE go_memstats_heap_objects gauge\n")
		f("go_memstats_heap_objects %d\n", ms.HeapObjects)

		f("# HELP go_memstats_gc_cpu_fraction Fraction of CPU time used by GC\n")
		f("# TYPE go_memstats_gc_cpu_fraction gauge\n")
		f("go_memstats_gc_cpu_fraction %s\n", strconv.FormatFloat(ms.GCCPUFraction, 'f', -1, 64))

		f("# HELP go_memstats_last_gc_time_seconds Unix timestamp of last GC\n")
		f("# TYPE go_memstats_last_gc_time_seconds gauge\n")
		f("go_memstats_last_gc_time_seconds %d\n", ms.LastGC/1e9)

		f("# HELP go_threads Number of OS threads created\n")
		f("# TYPE go_threads gauge\n")
		f("go_threads %d\n", runtime.NumCPU())
	}
}
