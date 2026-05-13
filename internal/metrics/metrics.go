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

		// Custom app metrics
		fmt.Fprintf(w, "# HELP plexishow_active_streams Current active ffmpeg streams\n")
		fmt.Fprintf(w, "# TYPE plexishow_active_streams gauge\n")
		fmt.Fprintf(w, "plexishow_active_streams %d\n", atomic.LoadInt64(&r.activeStreams))
		fmt.Fprintf(w, "# HELP plexishow_stream_errors_total Total stream errors\n")
		fmt.Fprintf(w, "# TYPE plexishow_stream_errors_total counter\n")
		fmt.Fprintf(w, "plexishow_stream_errors_total %d\n", atomic.LoadInt64(&r.streamErrors))

		// Go runtime metrics
		fmt.Fprintf(w, "# HELP go_goroutines Number of goroutines\n")
		fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
		fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())

		fmt.Fprintf(w, "# HELP go_memstats_alloc_bytes_bytes Number of bytes allocated and still in use\n")
		fmt.Fprintf(w, "# TYPE go_memstats_alloc_bytes_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_alloc_bytes_bytes %d\n", ms.Alloc)

		fmt.Fprintf(w, "# HELP go_memstats_sys_bytes Total bytes obtained from system\n")
		fmt.Fprintf(w, "# TYPE go_memstats_sys_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_sys_bytes %d\n", ms.Sys)

		fmt.Fprintf(w, "# HELP go_memstats_heap_alloc_bytes Heap allocation bytes\n")
		fmt.Fprintf(w, "# TYPE go_memstats_heap_alloc_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_heap_alloc_bytes %d\n", ms.HeapAlloc)

		fmt.Fprintf(w, "# HELP go_memstats_heap_inuse_bytes Heap in-use bytes\n")
		fmt.Fprintf(w, "# TYPE go_memstats_heap_inuse_bytes gauge\n")
		fmt.Fprintf(w, "go_memstats_heap_inuse_bytes %d\n", ms.HeapInuse)

		fmt.Fprintf(w, "# HELP go_memstats_heap_objects Number of allocated heap objects\n")
		fmt.Fprintf(w, "# TYPE go_memstats_heap_objects gauge\n")
		fmt.Fprintf(w, "go_memstats_heap_objects %d\n", ms.HeapObjects)

		fmt.Fprintf(w, "# HELP go_memstats_gc_cpu_fraction Fraction of CPU time used by GC\n")
		fmt.Fprintf(w, "# TYPE go_memstats_gc_cpu_fraction gauge\n")
		fmt.Fprintf(w, "go_memstats_gc_cpu_fraction %s\n", strconv.FormatFloat(ms.GCCPUFraction, 'f', -1, 64))

		fmt.Fprintf(w, "# HELP go_memstats_last_gc_time_seconds Unix timestamp of last GC\n")
		fmt.Fprintf(w, "# TYPE go_memstats_last_gc_time_seconds gauge\n")
		fmt.Fprintf(w, "go_memstats_last_gc_time_seconds %d\n", ms.LastGC/1e9)

		fmt.Fprintf(w, "# HELP go_threads Number of OS threads created\n")
		fmt.Fprintf(w, "# TYPE go_threads gauge\n")
		fmt.Fprintf(w, "go_threads %d\n", runtime.NumCPU())
	}
}
