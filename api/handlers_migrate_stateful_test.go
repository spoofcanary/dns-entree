package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	_ "github.com/spoofcanary/dns-entree/internal/fakeprovider"
	"github.com/spoofcanary/dns-entree/migrate"
)

// -----------------------------------------------------------------------------
// Test fake adapter (registered once via sync.Once for migrate.GetAdapter("fake")).
// EnsureZone calls are counted so we can assert preview does NOT touch the
// target.
// -----------------------------------------------------------------------------

type fakeAdapter struct {
	calls atomic.Int32
}

func (f *fakeAdapter) EnsureZone(ctx context.Context, domain string, opts migrate.ProviderOpts) (migrate.ZoneInfo, error) {
	f.calls.Add(1)
	return migrate.ZoneInfo{
		ZoneID:      "fake-zone-id",
		Nameservers: []string{"ns1.fake.test.", "ns2.fake.test."},
		Created:     true,
	}, nil
}

var (
	globalFakeAdapter *fakeAdapter
	fakeAdapterOnce   sync.Once
)

func ensureFakeAdapterRegistered() *fakeAdapter {
	fakeAdapterOnce.Do(func() {
		globalFakeAdapter = &fakeAdapter{}
		migrate.RegisterAdapter("fake", globalFakeAdapter)
	})
	return globalFakeAdapter
}

// newStatefulTestServer returns a Server wired with a memStore, a fixed 32-byte key,
// and a 1h TTL. The fake adapter is registered (once per process) so the
// "fake" target slug resolves.
func newStatefulTestServer(t *testing.T) (*Server, *memStore, *fakeAdapter) {
	t.Helper()
	adapter := ensureFakeAdapterRegistered()
	store := newMemStore()
	s := &Server{
		opts: Options{
			RequestTimeout: 30 * time.Second,
			Now:            time.Now,
		},
		logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
		metrics:                newMetrics(),
		mux:                    http.NewServeMux(),
		migrationStore:         store,
		migrationKey:           bytes.Repeat([]byte{0x42}, 32),
		migrationTTL:           time.Hour,
		migrationRatePerSecond: 1000, // effectively uncapped for tests
	}
	return s, store, adapter
}

// doStatefulJSON posts a JSON body to a handler and decodes the envelope. The Server
// handler is invoked directly (no mux) by dispatching on the path prefix.
func doStatefulJSON(t *testing.T, s *Server, method, path string, body any, bearer string) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	dispatch(s, w, req)
	resp := w.Result()
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var env map[string]any
	if len(data) > 0 {
		_ = json.Unmarshal(data, &env)
	}
	return resp.StatusCode, env
}

// dispatch routes to the right handler based on method+path. Mirrors what
// Plan 07-05 will register on the real mux.
func dispatch(s *Server, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case r.Method == http.MethodPost && path == "/v1/migrate/preview":
		s.handleMigratePreview(w, r)
	case r.Method == http.MethodPost && len(path) > len("/v1/migrate/") && hasSuffix(path, "/apply"):
		s.handleMigrateApply(w, r)
	case r.Method == http.MethodPost && hasSuffix(path, "/verify"):
		s.handleMigrateVerify(w, r)
	case r.Method == http.MethodGet && path == "/v1/migrate":
		s.handleMigrateList(w, r)
	case r.Method == http.MethodGet && len(path) > len("/v1/migrate/"):
		s.handleMigrateGet(w, r)
	case r.Method == http.MethodDelete && len(path) > len("/v1/migrate/"):
		s.handleMigrateDelete(w, r)
	default:
		http.NotFound(w, r)
	}
}

func hasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}

// envData pulls the "data" object out of the success envelope.
func envData(t *testing.T, env map[string]any) map[string]any {
	t.Helper()
	d, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("envelope has no data object: %v", env)
	}
	return d
}

// basePreviewReq builds a preview request with a preloaded zone so ScrapeZone
// is skipped (no DNS needed).
func basePreviewReq(domain, tenant string) migratePreviewRequest {
	return migratePreviewRequest{
		Domain:   domain,
		Target:   "fake",
		TenantID: tenant,
		TargetCredentials: bodyCredentials{
			APIToken: "test-token",
		},
		PreloadedZone: &migrate.Zone{
			Domain: domain,
			Records: []entree.Record{
				{Type: "A", Name: domain, Content: "192.0.2.1", TTL: 300},
				{Type: "TXT", Name: domain, Content: "v=spf1 -all", TTL: 300},
			},
			Source:      "iterated",
			Nameservers: []string{"ns1.src.test."},
		},
	}
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestPreviewHappyPath(t *testing.T) {
	s, store, adapter := newStatefulTestServer(t)
	before := adapter.calls.Load()

	status, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("example.com", "tenant-a"), "")
	if status != http.StatusOK {
		t.Fatalf("status=%d env=%v", status, env)
	}
	d := envData(t, env)
	if d["id"] == nil || d["id"] == "" {
		t.Errorf("missing id in response")
	}
	if d["access_token"] == nil || d["access_token"] == "" {
		t.Errorf("missing access_token")
	}
	if d["status"] != "preview" {
		t.Errorf("status=%v, want preview", d["status"])
	}
	if d["expires_at"] == nil {
		t.Errorf("missing expires_at")
	}

	// Preview must NOT have called EnsureZone.
	if got := adapter.calls.Load(); got != before {
		t.Errorf("adapter.EnsureZone called %d times during preview, want 0", got-before)
	}

	// Exactly one row in the store, in preview state.
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.m) != 1 {
		t.Fatalf("expected 1 row, got %d", len(store.m))
	}
	for _, r := range store.m {
		if r.Status != migrate.StatusPreview {
			t.Errorf("row status=%s, want preview", r.Status)
		}
	}
}

func TestPreviewMissingDomain(t *testing.T) {
	s, _, _ := newStatefulTestServer(t)
	req := basePreviewReq("", "t")
	status, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", req, "")
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d env=%v", status, env)
	}
}

func TestApplyRequiresBearer(t *testing.T) {
	s, _, _ := newStatefulTestServer(t)
	_, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("a.com", "t"), "")
	id := envData(t, env)["id"].(string)

	status, _ := doStatefulJSON(t, s, "POST", "/v1/migrate/"+id+"/apply", nil, "")
	if status != http.StatusUnauthorized {
		t.Errorf("missing token: status=%d, want 401", status)
	}
}

func TestApplyWrongBearer(t *testing.T) {
	s, _, _ := newStatefulTestServer(t)
	_, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("a.com", "t"), "")
	id := envData(t, env)["id"].(string)

	status, _ := doStatefulJSON(t, s, "POST", "/v1/migrate/"+id+"/apply", nil, "wrong-token-value-xxxxxxxxxxxxxxxxxxxxxx")
	if status != http.StatusUnauthorized {
		t.Errorf("wrong token: status=%d, want 401", status)
	}
}

func TestApplyHappyPath(t *testing.T) {
	s, store, adapter := newStatefulTestServer(t)
	before := adapter.calls.Load()

	_, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("a.com", "t"), "")
	d := envData(t, env)
	id := d["id"].(string)
	token := d["access_token"].(string)

	status, aenv := doStatefulJSON(t, s, "POST", "/v1/migrate/"+id+"/apply", nil, token)
	if status != http.StatusOK {
		t.Fatalf("apply: status=%d env=%v", status, aenv)
	}
	ad := envData(t, aenv)
	if ad["status"] != "awaiting_ns_change" {
		t.Errorf("status=%v, want awaiting_ns_change", ad["status"])
	}
	// Adapter must have been called exactly once during apply.
	if got := adapter.calls.Load(); got != before+1 {
		t.Errorf("EnsureZone calls=%d, want %d", got, before+1)
	}

	row := store.rawGet(id)
	if row.Status != migrate.StatusAwaitingNSChange {
		t.Errorf("stored status=%s", row.Status)
	}
	if row.Version != 3 {
		t.Errorf("version=%d, want 3 (create=1, applying=2, awaiting=3)", row.Version)
	}
	if len(row.ApplyResults) != 2 {
		t.Errorf("apply_results=%d, want 2", len(row.ApplyResults))
	}
}

func TestApplyConcurrentReturns409(t *testing.T) {
	s, _, _ := newStatefulTestServer(t)
	_, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("c.com", "t"), "")
	d := envData(t, env)
	id := d["id"].(string)
	token := d["access_token"].(string)

	var wg sync.WaitGroup
	var got200, got409 atomic.Int32
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			status, _ := doStatefulJSON(t, s, "POST", "/v1/migrate/"+id+"/apply", nil, token)
			switch status {
			case http.StatusOK:
				got200.Add(1)
			case http.StatusConflict:
				got409.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got200.Load()+got409.Load() != 2 {
		t.Fatalf("unexpected outcome: 200=%d 409=%d", got200.Load(), got409.Load())
	}
	if got200.Load() < 1 {
		t.Errorf("expected at least one 200, got %d", got200.Load())
	}
	if got409.Load() < 1 {
		t.Errorf("expected at least one 409, got %d", got409.Load())
	}
}

func TestVerifyStateOnly(t *testing.T) {
	// Full NS verification would need a real DNS server; fakeprovider has no
	// NS. We assert the endpoint returns a summary (matched may be 0) and
	// that state gates (401 / 409) work.
	s, store, _ := newStatefulTestServer(t)
	_, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("v.com", "t"), "")
	d := envData(t, env)
	id := d["id"].(string)
	token := d["access_token"].(string)

	// Verify before apply -> must still be allowed (status is preview, but
	// plan gates on applying/awaiting/verifying/complete). Expect 409.
	status, _ := doStatefulJSON(t, s, "POST", "/v1/migrate/"+id+"/verify", nil, token)
	if status != http.StatusConflict {
		t.Errorf("verify in preview state: status=%d, want 409", status)
	}

	// Apply, then verify.
	doStatefulJSON(t, s, "POST", "/v1/migrate/"+id+"/apply", nil, token)

	status, venv := doStatefulJSON(t, s, "POST", "/v1/migrate/"+id+"/verify", nil, token)
	if status != http.StatusOK {
		t.Fatalf("verify: status=%d env=%v", status, venv)
	}
	vd := envData(t, venv)
	if vd["total"] == nil {
		t.Errorf("missing total in verify response")
	}
	// NS resolution against fake.test will fail; status should be verifying.
	row := store.rawGet(id)
	if row.Status != migrate.StatusVerifying && row.Status != migrate.StatusComplete {
		t.Errorf("post-verify status=%s, want verifying or complete", row.Status)
	}
}

func TestGetRedacted(t *testing.T) {
	s, _, _ := newStatefulTestServer(t)
	_, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("g.com", "t"), "")
	d := envData(t, env)
	id := d["id"].(string)
	token := d["access_token"].(string)

	status, genv := doStatefulJSON(t, s, "GET", "/v1/migrate/"+id, nil, token)
	if status != http.StatusOK {
		t.Fatalf("get: status=%d", status)
	}
	gd := envData(t, genv)
	if v, ok := gd["credential_blob"]; ok && v != nil {
		t.Errorf("credential_blob not redacted: %v", v)
	}
	if v, ok := gd["access_token"].(string); ok && v != "" {
		t.Errorf("access_token not redacted: %q", v)
	}
}

func TestListFilterTenant(t *testing.T) {
	s, _, _ := newStatefulTestServer(t)
	doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("a.com", "tenant-a"), "")
	doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("b.com", "tenant-a"), "")
	doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("c.com", "tenant-b"), "")

	status, env := doStatefulJSON(t, s, "GET", "/v1/migrate?tenant_id=tenant-a", nil, "")
	if status != http.StatusOK {
		t.Fatalf("list: status=%d", status)
	}
	rows, _ := env["data"].([]any)
	if len(rows) != 2 {
		t.Errorf("tenant-a rows=%d, want 2", len(rows))
	}

	// All should have credential_blob and access_token redacted.
	for _, r := range rows {
		m := r.(map[string]any)
		if v, ok := m["credential_blob"]; ok && v != nil {
			t.Errorf("list row not redacted: %v", v)
		}
		if v, ok := m["access_token"].(string); ok && v != "" {
			t.Errorf("list row access_token not redacted: %q", v)
		}
	}
}

func TestDelete(t *testing.T) {
	s, store, _ := newStatefulTestServer(t)
	_, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("d.com", "t"), "")
	d := envData(t, env)
	id := d["id"].(string)
	token := d["access_token"].(string)

	status, _ := doStatefulJSON(t, s, "DELETE", "/v1/migrate/"+id, nil, token)
	if status != http.StatusOK {
		t.Errorf("delete: status=%d", status)
	}
	if store.rawGet(id) != nil {
		t.Errorf("row not deleted")
	}
	// Subsequent GET -> 404 (but auth loads first; token compare against
	// empty string = unauthorized). Accept 404 or 401.
	status, _ = doStatefulJSON(t, s, "GET", "/v1/migrate/"+id, nil, token)
	if status != http.StatusNotFound {
		t.Errorf("get after delete: status=%d, want 404", status)
	}
}

func TestExpiredRowReturns410(t *testing.T) {
	s, store, _ := newStatefulTestServer(t)
	_, env := doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("e.com", "t"), "")
	d := envData(t, env)
	id := d["id"].(string)
	token := d["access_token"].(string)

	// Force expiry.
	row := store.rawGet(id)
	row.ExpiresAt = time.Now().Add(-1 * time.Hour)
	store.rawPut(row)

	status, _ := doStatefulJSON(t, s, "GET", "/v1/migrate/"+id, nil, token)
	if status != http.StatusGone {
		t.Errorf("expired get: status=%d, want 410", status)
	}
	status, _ = doStatefulJSON(t, s, "POST", "/v1/migrate/"+id+"/apply", nil, token)
	if status != http.StatusGone {
		t.Errorf("expired apply: status=%d, want 410", status)
	}
}

func TestPreviewPathDoesNotCallEnsureZone(t *testing.T) {
	s, _, adapter := newStatefulTestServer(t)
	before := adapter.calls.Load()
	for i := 0; i < 5; i++ {
		doStatefulJSON(t, s, "POST", "/v1/migrate/preview", basePreviewReq("np.com", "t"), "")
	}
	if got := adapter.calls.Load(); got != before {
		t.Errorf("EnsureZone called %d times during preview, want 0", got-before)
	}
}
