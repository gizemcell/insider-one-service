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

// handleMetrics serves a Prometheus text-format counter for every
// (method, path, status) combination seen since startup.
func handleMetrics(w http.ResponseWriter, _ *http.Request) {
	metricsMu.Lock()
	snapshot := make(map[metricKey]uint64, len(reqTotals))
	for k, v := range reqTotals {
		snapshot[k] = v
	}
	metricsMu.Unlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintln(w, "# HELP http_requests_total Total number of HTTP requests.")
	fmt.Fprintln(w, "# TYPE http_requests_total counter")
	for k, v := range snapshot {
		fmt.Fprintf(w, "http_requests_total{method=%q,path=%q,status=%q} %d\n",
			k.method, k.path, k.status, v)
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

		recordRequest(r.Method, r.URL.Path, fmt.Sprintf("%d", sw.status))
		slog.Info("request",
			"request_id", id,
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
