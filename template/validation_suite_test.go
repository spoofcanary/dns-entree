//go:build templates
// +build templates

// Run with: go test -tags=templates ./template/...
// This suite hits the network to clone Domain-Connect/Templates (D-22).
// Regular `go test ./...` skips it because of the build tag.

package template

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAllOfficialTemplates(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "dc-templates")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := SyncTemplates(ctx, WithCacheDir(cacheDir)); err != nil {
		t.Fatalf("SyncTemplates: %v", err)
	}

	summaries, err := ListTemplates(WithCacheDir(cacheDir))
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(summaries) < 100 {
		t.Fatalf("expected >=100 templates, got %d", len(summaries))
	}

	t.Logf("loaded %d official templates from cache", len(summaries))

	for _, s := range summaries {
		s := s
		name := s.ProviderID + "." + s.ServiceID
		t.Run(name, func(t *testing.T) {
			tmpl, err := LoadTemplateFile(s.Path)
			if err != nil {
				t.Errorf("LoadTemplateFile %s: %v", s.Path, err)
				return
			}
			vars := syntheticVars(tmpl)
			records, err := tmpl.Resolve(vars)
			if err != nil {
				t.Errorf("Resolve %s: %v", name, err)
				return
			}
			if len(records) == 0 {
				// Some templates may legitimately contain only unsupported
				// record types; warn but don't fail.
				t.Logf("template %s emitted zero records (all skipped)", name)
				return
			}
			for i, r := range records {
				if r.Type == "" {
					t.Errorf("record %d: empty Type", i)
				}
				switch r.Type {
				case "TXT", "SPFM":
					if r.Content == "" {
						t.Errorf("record %d (%s): empty Content", i, r.Type)
					}
				case "A", "AAAA", "CNAME", "NS", "MX", "SRV":
					if r.Content == "" {
						t.Errorf("record %d (%s): empty Content", i, r.Type)
					}
				}
			}
		})
	}
}

// syntheticVars walks every template field that supports %var% substitution
// and returns a map populated with safe defaults for each unique variable.
func syntheticVars(t *Template) map[string]string {
	out := map[string]string{}
	collect := func(s string) {
		for _, m := range varRegex.FindAllStringSubmatch(s, -1) {
			name := m[1]
			if _, ok := out[name]; ok {
				continue
			}
			out[name] = syntheticValue(name)
		}
	}
	for _, r := range t.Records {
		collect(r.Host)
		collect(r.PointsTo)
		collect(r.Target)
		collect(r.Data)
		collect(r.TxtConflictMatchingPrefix)
	}
	return out
}

func syntheticValue(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "ipv6"), strings.Contains(n, "v6"):
		return "2001:db8::1"
	case strings.Contains(n, "ip"), strings.Contains(n, "address"):
		return "192.0.2.1"
	case strings.Contains(n, "email"), strings.Contains(n, "mail") && strings.Contains(n, "addr"):
		return "test.example.com"
	case strings.Contains(n, "domain"), strings.Contains(n, "host"), strings.Contains(n, "fqdn"), strings.Contains(n, "target"), strings.Contains(n, "cname"), strings.Contains(n, "mx"), strings.Contains(n, "ns"):
		return "test.example.com"
	case strings.Contains(n, "key"), strings.Contains(n, "token"), strings.Contains(n, "id"), strings.Contains(n, "secret"), strings.Contains(n, "code"):
		return "test-token-123"
	case strings.Contains(n, "port"):
		return "443"
	case strings.Contains(n, "priority"), strings.Contains(n, "weight"):
		return "10"
	default:
		return "test.example.com"
	}
}
