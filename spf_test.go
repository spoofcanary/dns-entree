package entree

import (
	"strings"
	"testing"
)

func TestMergeSPF(t *testing.T) {
	tests := []struct {
		name            string
		current         string
		includes        []string
		wantValue       string
		wantChanged     bool
		wantBroken      bool
		wantLookupCount int
		wantExceeded    bool
	}{
		{
			name:            "empty_no_includes",
			current:         "",
			includes:        nil,
			wantValue:       "v=spf1 ~all",
			wantChanged:     true,
			wantLookupCount: 0,
		},
		{
			name:            "empty_with_include",
			current:         "",
			includes:        []string{"a.com"},
			wantValue:       "v=spf1 include:a.com ~all",
			wantChanged:     true,
			wantLookupCount: 1,
		},
		{
			name:            "broken",
			current:         "junk",
			includes:        []string{"a.com"},
			wantValue:       "v=spf1 include:a.com ~all",
			wantChanged:     true,
			wantBroken:      true,
			wantLookupCount: 1,
		},
		{
			name:            "valid_fresh",
			current:         "v=spf1 -all",
			includes:        []string{"a.com"},
			wantValue:       "v=spf1 include:a.com -all",
			wantChanged:     true,
			wantLookupCount: 1,
		},
		{
			name:            "already_merged",
			current:         "v=spf1 include:a.com -all",
			includes:        []string{"a.com"},
			wantValue:       "v=spf1 include:a.com -all",
			wantChanged:     false,
			wantLookupCount: 1,
		},
		{
			name:            "preserves_ip4",
			current:         "v=spf1 ip4:1.2.3.4 -all",
			includes:        []string{"a.com"},
			wantValue:       "v=spf1 ip4:1.2.3.4 include:a.com -all",
			wantChanged:     true,
			wantLookupCount: 1,
		},
		{
			name:            "redirect_terminator",
			current:         "v=spf1 redirect=other.com",
			includes:        []string{"a.com"},
			wantValue:       "v=spf1 include:a.com redirect=other.com",
			wantChanged:     true,
			wantLookupCount: 2,
		},
		{
			name:            "mixed_lookups",
			current:         "v=spf1 a mx include:x.com -all",
			includes:        nil,
			wantValue:       "v=spf1 a mx include:x.com -all",
			wantChanged:     false,
			wantLookupCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MergeSPF(tt.current, tt.includes)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Value != tt.wantValue {
				t.Errorf("Value = %q, want %q", got.Value, tt.wantValue)
			}
			if got.Changed != tt.wantChanged {
				t.Errorf("Changed = %v, want %v", got.Changed, tt.wantChanged)
			}
			if got.BrokenInput != tt.wantBroken {
				t.Errorf("BrokenInput = %v, want %v", got.BrokenInput, tt.wantBroken)
			}
			if got.LookupCount != tt.wantLookupCount {
				t.Errorf("LookupCount = %d, want %d", got.LookupCount, tt.wantLookupCount)
			}
			if got.LookupLimitExceeded != tt.wantExceeded {
				t.Errorf("LookupLimitExceeded = %v, want %v", got.LookupLimitExceeded, tt.wantExceeded)
			}
		})
	}
}

func TestMergeSPF_Overflow(t *testing.T) {
	includes := []string{"a1.com", "a2.com", "a3.com", "a4.com", "a5.com", "a6.com", "a7.com", "a8.com", "a9.com", "a10.com", "a11.com"}
	got, err := MergeSPF("v=spf1 -all", includes)
	if err != nil {
		t.Fatal(err)
	}
	if got.LookupCount != 11 {
		t.Errorf("LookupCount = %d, want 11", got.LookupCount)
	}
	if !got.LookupLimitExceeded {
		t.Error("LookupLimitExceeded = false, want true")
	}
	if len(got.Warnings) == 0 {
		t.Error("expected warning for overflow")
	}
	if !strings.HasPrefix(got.Value, "v=spf1 ") || !strings.HasSuffix(got.Value, " -all") {
		t.Errorf("Value malformed: %q", got.Value)
	}
	for _, inc := range includes {
		if !strings.Contains(got.Value, "include:"+inc) {
			t.Errorf("missing include %s in %q", inc, got.Value)
		}
	}
}

func TestMergeSPF_Idempotent(t *testing.T) {
	first, err := MergeSPF("v=spf1 -all", []string{"a.com", "b.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Changed {
		t.Fatal("first pass should be Changed")
	}
	second, err := MergeSPF(first.Value, []string{"a.com", "b.com"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed {
		t.Errorf("second pass Changed = true, want false")
	}
	if second.Value != first.Value {
		t.Errorf("Value drift: %q vs %q", second.Value, first.Value)
	}
}

func TestMergeSPF_PreservesMechanisms(t *testing.T) {
	got, err := MergeSPF("v=spf1 ip4:1.2.3.4 include:a.com -all", []string{"b.com"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ip4:1.2.3.4", "include:a.com", "include:b.com", "-all"} {
		if !strings.Contains(got.Value, want) {
			t.Errorf("Value %q missing %q", got.Value, want)
		}
	}
}
