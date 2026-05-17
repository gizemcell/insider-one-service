package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		{"version", handleVersion, http.StatusOK, `{"version":"dev"}` + "\n", "application/json; charset=utf-8"},
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
