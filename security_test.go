package entree

import (
	"fmt"
	"strings"
	"testing"
)

// TestSecurityCredentialStructNoLeak verifies that the Credentials struct
// does not leak secrets when formatted with fmt verbs. Since Credentials has
// no String()/GoString()/Format() methods, the default %v output will show
// field values. This test documents that callers must NOT log Credentials
// via fmt functions. If a String() method is ever added, this test ensures it
// redacts sensitive fields.
func TestSecurityCredentialStructNoLeak(t *testing.T) {
	c := Credentials{
		APIToken:  "super-secret-token",
		APIKey:    "my-api-key",
		APISecret: "my-api-secret",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG",
		Token:     "oauth-bearer-token",
		ProjectID: "my-project", // not sensitive per se
		Region:    "us-east-1",  // not sensitive per se
	}

	// Verify the struct's sensitive fields exist (compile-time guard).
	sensitiveFields := []string{
		c.APIToken,
		c.APIKey,
		c.APISecret,
		c.AccessKey,
		c.SecretKey,
		c.Token,
	}

	for _, val := range sensitiveFields {
		if val == "" {
			t.Error("test setup: expected non-empty credential field")
		}
	}

	// If Credentials ever gets a String() method, it must redact secrets.
	type stringer interface {
		String() string
	}
	if s, ok := any(c).(stringer); ok {
		str := s.String()
		for _, val := range sensitiveFields {
			if val != "" && strings.Contains(str, val) {
				t.Errorf("Credentials.String() leaks secret %q", val)
			}
		}
	}

	// If Credentials ever gets a GoString() method, it must redact secrets.
	type goStringer interface {
		GoString() string
	}
	if gs, ok := any(c).(goStringer); ok {
		str := gs.GoString()
		for _, val := range sensitiveFields {
			if val != "" && strings.Contains(str, val) {
				t.Errorf("Credentials.GoString() leaks secret %q", val)
			}
		}
	}

	// Document that without String(), %v WILL show secrets. This is expected
	// behavior for a struct without a custom formatter. Callers must not log it.
	repr := fmt.Sprintf("%v", c)
	if repr == "" {
		t.Error("expected non-empty representation")
	}
}

// TestSecurityMergeSPFNoAlloc verifies MergeSPF does not panic on edge cases
// that could indicate security issues in the parser.
func TestSecurityMergeSPFEdgeCases(t *testing.T) {
	cases := []struct {
		name     string
		current  string
		includes []string
	}{
		{"empty_both", "", nil},
		{"empty_current", "", []string{"_spf.example.com"}},
		{"garbage_input", "not an spf record @#$%^&*()", []string{"include.com"}},
		{"null_bytes", "v=spf1 \x00 ~all", []string{"test.com"}},
		{"very_long", "v=spf1 " + strings.Repeat("include:a.com ", 100) + "~all", []string{"b.com"}},
		{"unicode", "v=spf1 include:\xc0\xaf.com ~all", []string{"test.com"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic.
			result, err := MergeSPF(tc.current, tc.includes)
			if err != nil {
				t.Fatalf("MergeSPF returned error: %v", err)
			}
			// Result should always have a value.
			if result.Value == "" {
				t.Error("MergeSPF returned empty Value")
			}
		})
	}
}
