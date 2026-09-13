package diag

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	runtimepprof "runtime/pprof"
	"sort"
	"strconv"
	"time"

	"go.uber.org/zap"
)

const (
	snapshotInterval   = 5 * time.Minute
	dumpThresholdBytes = 1228 * 1024 * 1024
	dumpRingSize       = 3
)

func Start(ctx context.Context, log *zap.SugaredLogger, port int, dumpDir string) {
	if port <= 0 {
		return
	}

	serve(ctx, log, port)

	go watch(ctx, log, dumpDir)
}

func serve(ctx context.Context, log *zap.SugaredLogger, port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:              net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warnw("diagnostic pprof server exited", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = srv.Shutdown(shutdownCtx)
	}()
}

func watch(ctx context.Context, log *zap.SugaredLogger, dumpDir string) {
	logSnapshot(log, dumpDir)

	ticker := time.NewTicker(snapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logSnapshot(log, dumpDir)
		}
	}
}

func logSnapshot(log *zap.SugaredLogger, dumpDir string) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	log.Infow(
		"diagnostic memory snapshot",
		"heap_alloc_mb", mem.HeapAlloc/1024/1024,
		"heap_inuse_mb", mem.HeapInuse/1024/1024,
		"sys_mb", mem.Sys/1024/1024,
		"num_gc", mem.NumGC,
		"goroutines", runtime.NumGoroutine(),
	)

	if mem.Sys < dumpThresholdBytes {
		return
	}

	log.Warnw("memory above dump threshold, writing profiles", "sys_mb", mem.Sys/1024/1024)

	stamp := time.Now().UTC().Format("20060102-150405")

	if err := writeProfiles(dumpDir, stamp); err != nil {
		log.Warnw("failed to write diagnostic profiles", "error", err)
	}
}

func writeProfiles(dumpDir, stamp string) error {
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		return err
	}

	pruneRing(dumpDir)

	heap, err := os.Create(filepath.Join(dumpDir, "heap-"+stamp+".pprof"))
	if err != nil {
		return err
	}

	heapErr := runtimepprof.WriteHeapProfile(heap)

	if closeErr := heap.Close(); heapErr == nil {
		heapErr = closeErr
	}

	if heapErr != nil {
		return heapErr
	}

	stacks, err := os.Create(filepath.Join(dumpDir, "goroutine-"+stamp+".txt"))
	if err != nil {
		return err
	}

	stackErr := runtimepprof.Lookup("goroutine").WriteTo(stacks, 2)

	if closeErr := stacks.Close(); stackErr == nil {
		stackErr = closeErr
	}

	return stackErr
}

func pruneRing(dumpDir string) {
	heaps, _ := filepath.Glob(filepath.Join(dumpDir, "heap-*.pprof"))
	pruneFiles(heaps)

	stacks, _ := filepath.Glob(filepath.Join(dumpDir, "goroutine-*.txt"))
	pruneFiles(stacks)
}

func pruneFiles(paths []string) {
	sort.Strings(paths)

	for len(paths) > dumpRingSize-1 {
		_ = os.Remove(paths[0])
		paths = paths[1:]
	}
}
