package entree

import (
	"strings"
	"testing"
)

func FuzzMergeSPF(f *testing.F) {
	// Seed corpus with representative inputs.
	f.Add("v=spf1 include:_spf.google.com ~all", "include:sendcanary.com")
	f.Add("", "include:example.com")
	f.Add("v=spf1 redirect=_spf.other.com", "include:new.com")
	f.Add("broken spf garbage @#$%", "include:test.com")
	f.Add("v=spf1 "+strings.Repeat("include:a.com ", 20)+"~all", "include:b.com")
	f.Add("v=spf1 -all", "")
	f.Add("v=spf1 ~all", "include:x.com")
	f.Add("v=spf1 +all", "include:y.com")
	f.Add("v=spf1 ?all", "include:z.com")

	f.Fuzz(func(t *testing.T, current, include string) {
		// Extract includes from the fuzzed string (treat as single include).
		var includes []string
		if include != "" {
			// Strip the "include:" prefix if the fuzzer added it.
			inc := strings.TrimPrefix(include, "include:")
			if inc != "" {
				includes = []string{inc}
			}
		}

		// Must never panic (implicit). Result should be usable.
		result, err := MergeSPF(current, includes)
		if err != nil {
			t.Fatalf("MergeSPF returned error: %v", err)
		}
		if result.Value == "" {
			t.Error("empty result value")
		}
	})
}
