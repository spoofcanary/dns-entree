package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T, opts Options) *httptest.Server {
	t.Helper()
	s := NewServer(opts)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestHealthz(t *testing.T) {
	ts := newTestServer(t, Options{})
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(string(body), `"schema_version":1`) {
		t.Fatalf("missing schema_version: %s", body)
	}
}

func TestReadyzDefault(t *testing.T) {
	ts := newTestServer(t, Options{})
	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestReadyzFailing(t *testing.T) {
	ts := newTestServer(t, Options{
		ReadyCheck: func(ctx context.Context) error { return errors.New("nope") },
	})
	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestOpenAPIServed(t *testing.T) {
	ts := newTestServer(t, Options{})
	resp, err := http.Get(ts.URL + "/v1/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/yaml" {
		t.Fatalf("content-type=%s", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(body), "openapi:") {
		t.Fatalf("body did not start with openapi:: %s", body[:min(40, len(body))])
	}
}

func TestGracefulShutdown(t *testing.T) {
	s := NewServer(Options{Listen: "127.0.0.1:0"})
	// Bind a real listener via httptest so we have a free port.
	ln := httptest.NewUnstartedServer(s.Handler())
	addr := ln.Listener.Addr().String()
	ln.Close()
	s.opts.Listen = addr

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.ListenAndServe(ctx) }()

	// Give the listener a beat to bind.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ListenAndServe returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ListenAndServe did not return within 2s of cancel")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
