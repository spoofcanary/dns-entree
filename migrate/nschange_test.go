package migrate

import (
	"strings"
	"testing"
)

func TestFormatNSChangeInstructions_GoDaddy(t *testing.T) {
	out := FormatNSChangeInstructions("godaddy", []string{"alice.ns.example.", "bob.ns.example."})
	for _, want := range []string{
		"GoDaddy",
		"dcc.godaddy.com",
		"alice.ns.example",
		"bob.ns.example",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestFormatNSChangeInstructions_Unknown(t *testing.T) {
	out := FormatNSChangeInstructions("wackynet", []string{"ns1.target.com"})
	if !strings.Contains(out, "wackynet") {
		t.Errorf("expected fallback to mention slug: %s", out)
	}
	if !strings.Contains(out, "ns1.target.com") {
		t.Errorf("expected new NS: %s", out)
	}
}

func TestFormatNSChangeInstructions_EmptyNS(t *testing.T) {
	out := FormatNSChangeInstructions("godaddy", nil)
	if !strings.Contains(out, "no nameservers returned") {
		t.Errorf("expected empty NS notice: %s", out)
	}
}

func TestWriteLimiter_ZeroDefaults(t *testing.T) {
	l := NewWriteLimiter(0)
	if l == nil || l.lim == nil {
		t.Fatal("expected non-nil limiter")
	}
}
