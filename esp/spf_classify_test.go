package esp

import (
	"context"
	"testing"
)

func TestClassifyFromSPF_TopLevelIncludes(t *testing.T) {
	spf := "v=spf1 include:_spf.google.com include:amazonses.com include:sendgrid.net ~all"
	results := ClassifyFromSPF(spf)
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	wantByName := map[string]Infrastructure{
		"Google Workspace": InfraGoogle,
		"Amazon SES":       InfraSES,
		"SendGrid":         InfraTwilio,
	}
	for _, r := range results {
		want, ok := wantByName[r.Name]
		if !ok {
			t.Errorf("unexpected sender %q", r.Name)
			continue
		}
		if r.Infrastructure != want {
			t.Errorf("%s infra=%q want %q", r.Name, r.Infrastructure, want)
		}
	}
}

func TestClassifyFromSPF_SkipsUnknown(t *testing.T) {
	spf := "v=spf1 include:unknown.example.com include:_spf.google.com ~all"
	results := ClassifyFromSPF(spf)
	if len(results) != 1 || results[0].Name != "Google Workspace" {
		t.Fatalf("results=%+v want only Google Workspace", results)
	}
}

// fakeSPFResolver returns predetermined TXT records per domain.
type fakeSPFResolver struct {
	records map[string][]string
}

func (f *fakeSPFResolver) LookupTXT(_ context.Context, domain string) ([]string, error) {
	return f.records[domain], nil
}

func TestClassifyFromSPFRecursive_SESInChain(t *testing.T) {
	// Simulates a domain using Resend, whose SPF chain eventually includes amazonses.com.
	resolver := &fakeSPFResolver{
		records: map[string][]string{
			"_spf.resend.com": {"v=spf1 include:amazonses.com ~all"},
			"amazonses.com":   {"v=spf1 ip4:54.240.0.0/18 ~all"},
		},
	}
	spf := "v=spf1 include:_spf.resend.com ~all"
	results := ClassifyFromSPFRecursive(context.Background(), resolver, spf)
	if len(results) != 1 {
		t.Fatalf("results=%+v", results)
	}
	r := results[0]
	if r.Name != "Resend" {
		t.Errorf("name=%q want Resend", r.Name)
	}
	if r.Infrastructure != InfraSES {
		t.Errorf("infra=%q want %q", r.Infrastructure, InfraSES)
	}
}

func TestClassifyFromSPFRecursive_UnknownIncludeWithSESChain(t *testing.T) {
	// Unknown top-level include but chain walks into amazonses.com; we should
	// still emit a Custom sender entry flagged as SES via chain.
	resolver := &fakeSPFResolver{
		records: map[string][]string{
			"custom.example.com": {"v=spf1 include:amazonses.com ~all"},
		},
	}
	spf := "v=spf1 include:custom.example.com ~all"
	results := ClassifyFromSPFRecursive(context.Background(), resolver, spf)
	if len(results) != 1 {
		t.Fatalf("results=%+v", results)
	}
	r := results[0]
	if r.Infrastructure != InfraSES {
		t.Errorf("infra=%q want %q", r.Infrastructure, InfraSES)
	}
	if !r.ViaChain {
		t.Errorf("ViaChain=false, want true")
	}
	if r.Category != CategoryUnknown {
		t.Errorf("category=%q want unknown", r.Category)
	}
}

func TestClassifyFromSPFRecursive_NoChainForKnownSES(t *testing.T) {
	// Known SES-backed ESP (Resend) should not trigger chain walk since
	// catalog already says SES.
	resolver := &fakeSPFResolver{
		records: map[string][]string{}, // empty; if chain were walked, lookup fails silently
	}
	spf := "v=spf1 include:_spf.resend.com ~all"
	results := ClassifyFromSPFRecursive(context.Background(), resolver, spf)
	if len(results) != 1 || results[0].Infrastructure != InfraSES {
		t.Fatalf("results=%+v", results)
	}
	if results[0].ViaChain {
		t.Errorf("ViaChain=true but catalog already classified")
	}
}

func TestClassifyFromSPFRecursive_CycleDoesNotHang(t *testing.T) {
	resolver := &fakeSPFResolver{
		records: map[string][]string{
			"loop.a": {"v=spf1 include:loop.b ~all"},
			"loop.b": {"v=spf1 include:loop.a ~all"},
		},
	}
	spf := "v=spf1 include:loop.a ~all"
	results := ClassifyFromSPFRecursive(context.Background(), resolver, spf)
	if len(results) != 1 {
		t.Fatalf("results=%+v", results)
	}
}

func TestExtractIncludes(t *testing.T) {
	cases := []struct {
		spf  string
		want []string
	}{
		{"v=spf1 include:a.com include:b.com ~all", []string{"a.com", "b.com"}},
		{"v=spf1 -include:bad.com include:good.com ~all", []string{"bad.com", "good.com"}},
		{"v=spf1 include:trailing.dot. ~all", []string{"trailing.dot"}},
		{"v=spf1 ip4:1.2.3.4 ~all", nil},
		{"", nil},
	}
	for _, tc := range cases {
		got := extractIncludes(tc.spf)
		if len(got) != len(tc.want) {
			t.Errorf("extractIncludes(%q) = %v, want %v", tc.spf, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("extractIncludes(%q)[%d] = %q, want %q", tc.spf, i, got[i], tc.want[i])
			}
		}
	}
}
