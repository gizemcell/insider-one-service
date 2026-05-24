package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandlers(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		wantStatus  int
		wantBody    string
		wantContent string
	}{
		{"ping", handlePing, http.StatusOK, "pong\n", "text/plain; charset=utf-8"},
		{"healthz", handleHealthz, http.StatusOK, `{"status":"ok"}` + "\n", "application/json; charset=utf-8"},
		{"version", handleVersion, http.StatusOK, fmt.Sprintf("{\"version\":%q}\n", version), "application/json; charset=utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/"+tt.name, nil)

			tt.handler(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
			if ct := res.Header.Get("Content-Type"); ct != tt.wantContent {
				t.Errorf("Content-Type = %q, want %q", ct, tt.wantContent)
			}
			body, _ := io.ReadAll(res.Body)
			if got := string(body); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}

// TestRoutingMethodGuard confirms the Go 1.22 method-aware mux rejects
// non-GET requests to these endpoints.
func TestRoutingMethodGuard(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handlePing)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ping", strings.NewReader(""))
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /ping status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleMetrics(t *testing.T) {
	metricsMu.Lock()
	reqTotals = map[metricKey]uint64{}
	metricsMu.Unlock()
	latencyMu.Lock()
	latencyHist = map[latencyKey]*latencyData{}
	latencyMu.Unlock()

	// Seed one latency observation so the histogram block appears.
	recordLatency("GET", "/ping", 5*time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	handleMetrics(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	body, _ := io.ReadAll(res.Body)
	s := string(body)
	if !strings.Contains(s, "http_requests_total") {
		t.Errorf("body missing http_requests_total:\n%s", s)
	}
	if !strings.Contains(s, "http_request_duration_seconds_bucket") {
		t.Errorf("body missing http_request_duration_seconds_bucket:\n%s", s)
	}
	if !strings.Contains(s, "http_request_duration_seconds_sum") {
		t.Errorf("body missing http_request_duration_seconds_sum:\n%s", s)
	}
	if !strings.Contains(s, "http_request_duration_seconds_count") {
		t.Errorf("body missing http_request_duration_seconds_count:\n%s", s)
	}
}

func TestLogRequestsRecordsMetrics(t *testing.T) {
	metricsMu.Lock()
	reqTotals = map[metricKey]uint64{}
	metricsMu.Unlock()
	latencyMu.Lock()
	latencyHist = map[latencyKey]*latencyData{}
	latencyMu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handlePing)
	handler := logRequests(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	handler.ServeHTTP(rec, req)

	metricsMu.Lock()
	count := reqTotals[metricKey{"GET", "/ping", "200"}]
	metricsMu.Unlock()


	if count != 1 {
		t.Errorf("want 1 request recorded, got %d", count)
	}
}

func TestLogRequestsAttachesRequestID(t *testing.T) {
	var gotID string

	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if id, ok := r.Context().Value(ctxKey{}).(string); ok {
			gotID = id
		}
	})
	handler := logRequests(inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	handler.ServeHTTP(rec, req)

	if len(gotID) != 16 {
		t.Errorf("request_id = %q, want 16-char hex string", gotID)
	}
}
