package entree

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeSPFResolver returns canned TXT responses keyed by domain.
type fakeSPFResolver struct {
	zones map[string][]string
	calls map[string]int
}

func (f *fakeSPFResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[domain]++
	txts, ok := f.zones[domain]
	if !ok {
		return nil, errors.New("NXDOMAIN")
	}
	return txts, nil
}

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

func TestMergeSPF_RecursiveLookupCount(t *testing.T) {
	// Build a nested tree:
	//   apex:           v=spf1 include:l1.com include:flat.com -all           (surface=2)
	//   l1.com:         v=spf1 include:l2.com a mx -all                       (adds 3)
	//   l2.com:         v=spf1 include:l3.com ip4:1.2.3.4 -all                (adds 1)
	//   l3.com:         v=spf1 a:host.l3.com -all                             (adds 1)
	//   flat.com:       v=spf1 -all                                           (adds 0)
	// Total recursive = 2 + 3 + 1 + 1 + 0 = 7
	fake := &fakeSPFResolver{zones: map[string][]string{
		"l1.com":   {"v=spf1 include:l2.com a mx -all"},
		"l2.com":   {"v=spf1 include:l3.com ip4:1.2.3.4 -all"},
		"l3.com":   {"v=spf1 a:host.l3.com -all"},
		"flat.com": {"v=spf1 -all"},
	}}
	got, err := MergeSPF("v=spf1 include:l1.com include:flat.com -all", nil,
		WithRecursiveLookupCount(fake))
	if err != nil {
		t.Fatal(err)
	}
	if got.LookupCount != 7 {
		t.Fatalf("recursive LookupCount = %d, want 7", got.LookupCount)
	}
	if got.LookupLimitExceeded {
		t.Fatal("should not exceed limit at 7")
	}

	// Surface count (no resolver) should be much smaller.
	surface, _ := MergeSPF("v=spf1 include:l1.com include:flat.com -all", nil)
	if surface.LookupCount != 2 {
		t.Fatalf("surface LookupCount = %d, want 2", surface.LookupCount)
	}
}

func TestMergeSPF_RecursiveCountExceedsLimit(t *testing.T) {
	// Two includes that each pull in 6 mechanisms -> 2 + 12 = 14.
	fake := &fakeSPFResolver{zones: map[string][]string{
		"heavy1.com": {"v=spf1 a mx a:x1 mx:x2 ptr:x3 exists:x4 -all"},
		"heavy2.com": {"v=spf1 a mx a:x1 mx:x2 ptr:x3 exists:x4 -all"},
	}}
	got, _ := MergeSPF("v=spf1 include:heavy1.com include:heavy2.com -all", nil,
		WithRecursiveLookupCount(fake))
	if got.LookupCount != 14 {
		t.Fatalf("LookupCount = %d, want 14", got.LookupCount)
	}
	if !got.LookupLimitExceeded {
		t.Fatal("expected LookupLimitExceeded=true for count 14")
	}
}

func TestMergeSPF_RecursiveResolverError_FallsBackToSurface(t *testing.T) {
	fake := &fakeSPFResolver{zones: map[string][]string{}} // nothing — NXDOMAIN for all
	got, _ := MergeSPF("v=spf1 include:missing.com -all", nil,
		WithRecursiveLookupCount(fake))
	// Surface count = 1 (the include mechanism itself).
	if got.LookupCount != 1 {
		t.Fatalf("fallback LookupCount = %d, want 1", got.LookupCount)
	}
	foundWarn := false
	for _, w := range got.Warnings {
		if strings.Contains(w, "recursive SPF lookup count incomplete") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Fatalf("expected fallback warning, got %v", got.Warnings)
	}
}

func TestMergeSPF_RecursiveCycleSafe(t *testing.T) {
	// a -> b -> a cycle; must terminate without blowing the depth limit.
	fake := &fakeSPFResolver{zones: map[string][]string{
		"a.com": {"v=spf1 include:b.com -all"},
		"b.com": {"v=spf1 include:a.com -all"},
	}}
	got, err := MergeSPF("v=spf1 include:a.com -all", nil,
		WithRecursiveLookupCount(fake))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// apex include:a.com (1) + a.com's include:b.com (1) + b.com's include:a.com (1, seen shortcut skips recurse)
	if got.LookupCount != 3 {
		t.Fatalf("cycle LookupCount = %d, want 3", got.LookupCount)
	}
	if fake.calls["a.com"] != 1 || fake.calls["b.com"] != 1 {
		t.Fatalf("expected each domain fetched once, got %+v", fake.calls)
	}
}

func TestNewNetSPFResolver_Construction(t *testing.T) {
	r := NewNetSPFResolver(0)
	if r == nil {
		t.Fatal("nil resolver")
	}
}
