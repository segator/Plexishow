package metrics

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"sync/atomic"
)

type Registry struct {
	activeStreams int64
	streamErrors  int64
}

func New() *Registry {
	return &Registry{}
}

func (r *Registry) IncActive() { atomic.AddInt64(&r.activeStreams, 1) }
func (r *Registry) DecActive() { atomic.AddInt64(&r.activeStreams, -1) }
func (r *Registry) IncErrors() { atomic.AddInt64(&r.streamErrors, 1) }

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
