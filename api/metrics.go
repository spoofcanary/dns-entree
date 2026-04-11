package api

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// defaultBuckets are the histogram boundaries (seconds) for HTTP request
// latency. Tuned for DNS API calls: sub-millisecond health checks up through
// long-running migrate operations.
var defaultBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30,
}

// metrics is a tiny pure-stdlib Prometheus registry. We avoid the official
// client to keep the binary dependency-free (D-24).
type metrics struct {
	mu sync.Mutex

	httpRequests   map[string]uint64   // key: method|route|status
	httpDurBuckets map[string][]uint64 // key: method|route, cumulative counts
	httpDurSum     map[string]float64
	httpDurCount   map[string]uint64
	providerOps    map[string]uint64 // key: provider|op|status
}

func newMetrics() *metrics {
	return &metrics{
		httpRequests:   map[string]uint64{},
		httpDurBuckets: map[string][]uint64{},
		httpDurSum:     map[string]float64{},
		httpDurCount:   map[string]uint64{},
		providerOps:    map[string]uint64{},
	}
}

func (m *metrics) observeHTTP(method, route string, status int, dur time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ck := method + "|" + route + "|" + strconv.Itoa(status)
	m.httpRequests[ck]++

	hk := method + "|" + route
	bs, ok := m.httpDurBuckets[hk]
	if !ok {
		bs = make([]uint64, len(defaultBuckets))
		m.httpDurBuckets[hk] = bs
	}
	secs := dur.Seconds()
	for i, b := range defaultBuckets {
		if secs <= b {
			bs[i]++
		}
	}
	m.httpDurSum[hk] += secs
	m.httpDurCount[hk]++
}

// IncProviderOp increments the provider operation counter. Wave-2 handlers
// call this to record per-provider success/failure rates.
func (m *metrics) IncProviderOp(provider, op, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providerOps[provider+"|"+op+"|"+status]++
}

func escapeLabelValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return v
}

func sortKeys(m map[string]uint64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortBucketKeys(m map[string][]uint64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// WriteText emits the registry in Prometheus text exposition format 0.0.4.
func (m *metrics) WriteText(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fmt.Fprintln(w, "# HELP entree_http_requests_total Total HTTP requests processed.")
	fmt.Fprintln(w, "# TYPE entree_http_requests_total counter")
	for _, k := range sortKeys(m.httpRequests) {
		parts := strings.SplitN(k, "|", 3)
		fmt.Fprintf(w, "entree_http_requests_total{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n",
			escapeLabelValue(parts[0]), escapeLabelValue(parts[1]), parts[2], m.httpRequests[k])
	}

	fmt.Fprintln(w, "# HELP entree_http_request_duration_seconds HTTP request duration in seconds.")
	fmt.Fprintln(w, "# TYPE entree_http_request_duration_seconds histogram")
	for _, k := range sortBucketKeys(m.httpDurBuckets) {
		parts := strings.SplitN(k, "|", 2)
		method, path := escapeLabelValue(parts[0]), escapeLabelValue(parts[1])
		buckets := m.httpDurBuckets[k]
		for i, b := range defaultBuckets {
			fmt.Fprintf(w, "entree_http_request_duration_seconds_bucket{method=\"%s\",path=\"%s\",le=\"%s\"} %d\n",
				method, path, strconv.FormatFloat(b, 'g', -1, 64), buckets[i])
		}
		fmt.Fprintf(w, "entree_http_request_duration_seconds_bucket{method=\"%s\",path=\"%s\",le=\"+Inf\"} %d\n",
			method, path, m.httpDurCount[k])
		fmt.Fprintf(w, "entree_http_request_duration_seconds_sum{method=\"%s\",path=\"%s\"} %s\n",
			method, path, strconv.FormatFloat(m.httpDurSum[k], 'g', -1, 64))
		fmt.Fprintf(w, "entree_http_request_duration_seconds_count{method=\"%s\",path=\"%s\"} %d\n",
			method, path, m.httpDurCount[k])
	}

	fmt.Fprintln(w, "# HELP entree_provider_operations_total Provider API operations invoked.")
	fmt.Fprintln(w, "# TYPE entree_provider_operations_total counter")
	for _, k := range sortKeys(m.providerOps) {
		parts := strings.SplitN(k, "|", 3)
		fmt.Fprintf(w, "entree_provider_operations_total{provider=\"%s\",op=\"%s\",status=\"%s\"} %d\n",
			escapeLabelValue(parts[0]), escapeLabelValue(parts[1]), escapeLabelValue(parts[2]), m.providerOps[k])
	}
}

func (m *metrics) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	m.WriteText(w)
}
