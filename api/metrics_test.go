package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMetricsExposition(t *testing.T) {
	ts := newTestServer(t, Options{})
	for i := 0; i < 3; i++ {
		resp, err := http.Get(ts.URL + "/healthz")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)

	if !strings.Contains(text, "entree_http_requests_total") {
		t.Fatalf("missing requests_total: %s", text)
	}
	if !strings.Contains(text, "entree_http_request_duration_seconds_bucket") {
		t.Fatalf("missing histogram: %s", text)
	}
	if !strings.Contains(text, "entree_provider_operations_total") {
		t.Fatalf("missing provider_ops: %s", text)
	}
	if !strings.Contains(text, `path="GET /healthz"`) {
		t.Fatalf("expected GET /healthz route label: %s", text)
	}
}

func TestMetricsCardinalityBoundedByRoute(t *testing.T) {
	ts := newTestServer(t, Options{})
	// Hit /healthz with various query strings — must collapse to one series.
	for _, q := range []string{"", "?a=1", "?a=2", "?b=foo"} {
		resp, _ := http.Get(ts.URL + "/healthz" + q)
		if resp != nil {
			resp.Body.Close()
		}
	}
	resp, _ := http.Get(ts.URL + "/metrics")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	count := strings.Count(string(body), `entree_http_requests_total{method="GET",path="GET /healthz",status="200"}`)
	if count != 1 {
		t.Fatalf("expected exactly 1 series for /healthz, got %d. body=%s", count, body)
	}
}
