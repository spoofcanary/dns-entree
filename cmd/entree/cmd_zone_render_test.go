package main

import (
	"strings"
	"testing"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/migrate"
)

func TestRenderBIND_TXTEscapesBackslashBeforeQuote(t *testing.T) {
	z := &migrate.Zone{
		Domain: "example.com",
		Source: "test",
		Records: []entree.Record{
			{Type: "TXT", Name: "example.com", Content: `a\b"c`, TTL: 300},
		},
	}
	out := renderBIND(z)
	// Expect the backslash doubled and the quote escaped: a\\b\"c
	want := `"a\\b\"c"`
	if !strings.Contains(out, want) {
		t.Errorf("renderBIND output missing %s\ngot:\n%s", want, out)
	}
	// Must NOT contain a bare unescaped backslash followed by b (the old
	// broken output would produce "a\b\"c").
	if strings.Contains(out, `"a\b`) {
		t.Errorf("renderBIND left backslash unescaped:\n%s", out)
	}
}
