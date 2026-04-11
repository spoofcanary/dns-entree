package template

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// makeFixtureRepo builds a non-bare git repo containing a single template file
// and returns a file:// URL pointing at it.
func makeFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	provDir := filepath.Join(dir, "foo")
	if err := os.MkdirAll(provDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonContent := `{
  "providerId": "foo",
  "providerName": "Foo Inc",
  "serviceId": "bar",
  "serviceName": "Bar Service",
  "version": 1,
  "records": [
    {"type": "TXT", "host": "@", "data": "hello", "ttl": 300}
  ]
}`
	if err := os.WriteFile(filepath.Join(provDir, "foo.bar.json"), []byte(jsonContent), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("foo/foo.bar.json"); err != nil {
		t.Fatal(err)
	}
	_, err = w.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "t@t", When: time.Now()},
	})
	if err != nil {
		t.Fatal(err)
	}
	u := &url.URL{Scheme: "file", Path: dir}
	return u.String()
}

func TestSyncTemplates_CloneAndIdempotent(t *testing.T) {
	repoURL := makeFixtureRepo(t)
	cache := filepath.Join(t.TempDir(), "cache")
	ctx := context.Background()

	if err := SyncTemplates(ctx, WithCacheDir(cache), WithRepoURL(repoURL)); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cache, ".git")); err != nil {
		t.Fatalf(".git missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cache, "foo", "foo.bar.json")); err != nil {
		t.Fatalf("template missing: %v", err)
	}
	if err := SyncTemplates(ctx, WithCacheDir(cache), WithRepoURL(repoURL)); err != nil {
		t.Fatalf("second sync: %v", err)
	}
}

func TestLoadTemplate_ReadsFromCache(t *testing.T) {
	repoURL := makeFixtureRepo(t)
	cache := filepath.Join(t.TempDir(), "cache")
	ctx := context.Background()
	if err := SyncTemplates(ctx, WithCacheDir(cache), WithRepoURL(repoURL)); err != nil {
		t.Fatal(err)
	}
	tmpl, err := LoadTemplate(ctx, "foo", "bar",
		WithCacheDir(cache), WithRepoURL(repoURL), WithCacheTTL(-1))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if tmpl.ProviderID != "foo" || tmpl.ServiceID != "bar" {
		t.Errorf("got %+v", tmpl)
	}
}

func TestLoadTemplate_NotFound(t *testing.T) {
	repoURL := makeFixtureRepo(t)
	cache := filepath.Join(t.TempDir(), "cache")
	ctx := context.Background()
	if err := SyncTemplates(ctx, WithCacheDir(cache), WithRepoURL(repoURL)); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTemplate(ctx, "nope", "missing",
		WithCacheDir(cache), WithRepoURL(repoURL), WithCacheTTL(-1))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadTemplate_RejectsBadID(t *testing.T) {
	ctx := context.Background()
	cache := t.TempDir()
	if _, err := LoadTemplate(ctx, "../etc", "passwd", WithCacheDir(cache), WithCacheTTL(-1)); err == nil {
		t.Error("expected providerID rejection")
	}
	if _, err := LoadTemplate(ctx, "ok", "../bad", WithCacheDir(cache), WithCacheTTL(-1)); err == nil {
		t.Error("expected serviceID rejection")
	}
}

func TestListTemplates(t *testing.T) {
	repoURL := makeFixtureRepo(t)
	cache := filepath.Join(t.TempDir(), "cache")
	ctx := context.Background()
	if err := SyncTemplates(ctx, WithCacheDir(cache), WithRepoURL(repoURL)); err != nil {
		t.Fatal(err)
	}
	list, err := ListTemplates(WithCacheDir(cache))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1, got %d: %+v", len(list), list)
	}
	if list[0].ProviderID != "foo" || list[0].ServiceID != "bar" {
		t.Errorf("got %+v", list[0])
	}
}

func TestCacheStale(t *testing.T) {
	cache := t.TempDir()
	// No sentinel -> stale
	if !cacheStale(cache, time.Hour) {
		t.Error("missing sentinel should be stale")
	}
	// Negative TTL -> never stale
	if cacheStale(cache, -1) {
		t.Error("negative ttl should never be stale")
	}
	// Zero TTL -> always stale
	if !cacheStale(cache, 0) {
		t.Error("zero ttl should always be stale")
	}
	// Fresh sentinel -> not stale
	sent := filepath.Join(cache, sentinelFile)
	if err := os.WriteFile(sent, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if cacheStale(cache, time.Hour) {
		t.Error("fresh sentinel should not be stale")
	}
	// Old sentinel -> stale
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(sent, old, old); err != nil {
		t.Fatal(err)
	}
	if !cacheStale(cache, time.Hour) {
		t.Error("old sentinel should be stale")
	}
}

func TestLoadTemplate_TTLZeroForcesRefresh(t *testing.T) {
	repoURL := makeFixtureRepo(t)
	cache := filepath.Join(t.TempDir(), "cache")
	ctx := context.Background()
	// First load with ttl=0 must clone (no prior cache).
	_, err := LoadTemplate(ctx, "foo", "bar",
		WithCacheDir(cache), WithRepoURL(repoURL), WithCacheTTL(0))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// Second load with ttl=0 still works (idempotent pull).
	_, err = LoadTemplate(ctx, "foo", "bar",
		WithCacheDir(cache), WithRepoURL(repoURL), WithCacheTTL(0))
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
}

func TestDefaultCacheDir_HonorsXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-test-dns-entree")
	d, err := defaultCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/xdg-test-dns-entree", "dns-entree", "templates")
	if d != want {
		t.Errorf("got %q want %q", d, want)
	}
}
