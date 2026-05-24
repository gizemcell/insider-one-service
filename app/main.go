package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var version = "v1.0.0"

// ctxKey is the unexported context key for request IDs.
type ctxKey struct{}

// metricKey identifies a unique combination of method, path, and status.
type metricKey struct{ method, path, status string }

var (
	metricsMu sync.Mutex
	reqTotals = map[metricKey]uint64{}
)

func recordRequest(method, path, status string) {
	k := metricKey{method, path, status}
	metricsMu.Lock()
	reqTotals[k]++
	metricsMu.Unlock()
}

// latencyBuckets are the histogram upper bounds in seconds (standard Prometheus defaults).
var latencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type latencyKey struct{ method, path string }

type latencyData struct {
	buckets []uint64 // cumulative count per latencyBuckets[i] upper bound
	sum     float64
	count   uint64
}

var (
	latencyMu   sync.Mutex
	latencyHist = map[latencyKey]*latencyData{}
)

func recordLatency(method, path string, dur time.Duration) {
	secs := dur.Seconds()
	k := latencyKey{method, path}
	latencyMu.Lock()
	d := latencyHist[k]
	if d == nil {
		d = &latencyData{buckets: make([]uint64, len(latencyBuckets))}
		latencyHist[k] = d
	}
	for i, le := range latencyBuckets {
		if secs <= le {
			d.buckets[i]++
		}
	}
	d.sum += secs
	d.count++
	latencyMu.Unlock()
}

// statusWriter wraps ResponseWriter to capture the written status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			return a
		},
	}))
	slog.SetDefault(logger)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	if v := os.Getenv("VERSION"); v != "" {
		version = v
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handlePing)
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /version", handleVersion)
	mux.HandleFunc("GET /metrics", handleMetrics)

	srv := &http.Server{
		Addr:              addr,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("stopped")
}

func handlePing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "pong")
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"status":"ok"}`)
}

func handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "{\"version\":%q}\n", version)
}

// handleMetrics serves Prometheus text-format metrics: request counters and
// latency histograms for every (method, path) combination seen since startup.
func handleMetrics(w http.ResponseWriter, _ *http.Request) {
	metricsMu.Lock()
	reqSnap := make(map[metricKey]uint64, len(reqTotals))
	for k, v := range reqTotals {
		reqSnap[k] = v
	}
	metricsMu.Unlock()

	latencyMu.Lock()
	latSnap := make(map[latencyKey]*latencyData, len(latencyHist))
	for k, d := range latencyHist {
		cp := &latencyData{
			buckets: make([]uint64, len(d.buckets)),
			sum:     d.sum,
			count:   d.count,
		}
		copy(cp.buckets, d.buckets)
		latSnap[k] = cp
	}
	latencyMu.Unlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	fmt.Fprintln(w, "# HELP http_requests_total Total number of HTTP requests.")
	fmt.Fprintln(w, "# TYPE http_requests_total counter")
	for k, v := range reqSnap {
		fmt.Fprintf(w, "http_requests_total{method=%q,path=%q,status=%q} %d\n",
			k.method, k.path, k.status, v)
	}

	fmt.Fprintln(w, "# HELP http_request_duration_seconds HTTP request latency histogram.")
	fmt.Fprintln(w, "# TYPE http_request_duration_seconds histogram")
	for k, d := range latSnap {
		for i, le := range latencyBuckets {
			fmt.Fprintf(w, "http_request_duration_seconds_bucket{method=%q,path=%q,le=\"%g\"} %d\n",
				k.method, k.path, le, d.buckets[i])
		}
		fmt.Fprintf(w, "http_request_duration_seconds_bucket{method=%q,path=%q,le=\"+Inf\"} %d\n",
			k.method, k.path, d.count)
		fmt.Fprintf(w, "http_request_duration_seconds_sum{method=%q,path=%q} %g\n",
			k.method, k.path, d.sum)
		fmt.Fprintf(w, "http_request_duration_seconds_count{method=%q,path=%q} %d\n",
			k.method, k.path, d.count)
	}
}

// logRequests attaches a request_id to each request, records it in the
// metrics counter, and emits a structured JSON access-log line.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("%016x", rand.Uint64())
		ctx := context.WithValue(r.Context(), ctxKey{}, id)
		r = r.WithContext(ctx)

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)
		dur := time.Since(start)

		recordRequest(r.Method, r.URL.Path, fmt.Sprintf("%d", sw.status))
		recordLatency(r.Method, r.URL.Path, dur)
		slog.Info("request",
			"request_id", id,
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", dur.Milliseconds(),
		)
	})
}
