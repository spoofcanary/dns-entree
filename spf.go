package entree

import (
	"context"
	"net"
	"strings"
	"time"
)

// MergeResult is the result of merging includes into an SPF record.
type MergeResult struct {
	Value               string
	Changed             bool
	BrokenInput         bool
	LookupCount         int
	LookupLimitExceeded bool
	Warnings            []string
}

// SPFResolver looks up TXT records for a domain. It is the abstraction used by
// WithRecursiveLookupCount to walk nested include: mechanisms when computing
// the true RFC 7208 lookup count. Implementations typically wrap a DNS
// resolver (see NewNetSPFResolver) but tests may supply fakes.
type SPFResolver interface {
	LookupTXT(ctx context.Context, domain string) ([]string, error)
}

// mergeOptions holds optional MergeSPF behavior toggles populated by
// MergeSPFOption functions.
type mergeOptions struct {
	resolver SPFResolver
}

// MergeSPFOption configures optional MergeSPF behavior.
type MergeSPFOption func(*mergeOptions)

// WithRecursiveLookupCount enables full RFC 7208 lookup counting. When set,
// MergeSPF recursively resolves every include: target (and redirect=) via
// resolver, sums their nested counts, and reports the total in
// MergeResult.LookupCount. Recursion is depth-limited to 10 per RFC 7208 to
// avoid runaway include chains. When unset (the default), LookupCount is a
// fast surface count with no network calls.
func WithRecursiveLookupCount(resolver SPFResolver) MergeSPFOption {
	return func(o *mergeOptions) {
		o.resolver = resolver
	}
}

// netSPFResolver adapts net.Resolver to the SPFResolver interface.
type netSPFResolver struct {
	r       *net.Resolver
	timeout time.Duration
}

// NewNetSPFResolver returns an SPFResolver backed by the stdlib net.Resolver
// with a per-lookup timeout (2s default if zero is passed).
func NewNetSPFResolver(timeout time.Duration) SPFResolver {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &netSPFResolver{r: net.DefaultResolver, timeout: timeout}
}

func (n *netSPFResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()
	return n.r.LookupTXT(ctx, domain)
}

// MergeSPF merges the given include domains into current, preserving all
// existing mechanisms. Idempotent: re-running on its own output with the same
// includes returns Changed=false. Caller is responsible for validating include
// strings -- they are passed through verbatim.
func MergeSPF(current string, includes []string, opts ...MergeSPFOption) (MergeResult, error) {
	var mo mergeOptions
	for _, opt := range opts {
		opt(&mo)
	}
	var res MergeResult
	trimmed := strings.TrimSpace(current)

	buildFresh := func() string {
		if len(includes) == 0 {
			return "v=spf1 ~all"
		}
		parts := make([]string, 0, len(includes)+2)
		parts = append(parts, "v=spf1")
		for _, inc := range includes {
			parts = append(parts, "include:"+inc)
		}
		parts = append(parts, "~all")
		return strings.Join(parts, " ")
	}

	switch {
	case trimmed == "":
		res.Value = buildFresh()
		res.Changed = true
	case !strings.HasPrefix(strings.ToLower(trimmed), "v=spf1"):
		res.Value = buildFresh()
		res.Changed = true
		res.BrokenInput = true
		res.Warnings = append(res.Warnings, "existing SPF was unparseable; replaced with fresh record")
	default:
		var additions []string
		for _, inc := range includes {
			if !strings.Contains(trimmed, "include:"+inc) {
				additions = append(additions, "include:"+inc)
			}
		}
		if len(additions) == 0 {
			res.Value = trimmed
			res.Changed = false
		} else {
			insert := " " + strings.Join(additions, " ")
			terminators := []string{" -all", " ~all", " ?all", " +all"}
			inserted := false
			for _, term := range terminators {
				if idx := strings.Index(trimmed, term); idx >= 0 {
					res.Value = trimmed[:idx] + insert + trimmed[idx:]
					inserted = true
					break
				}
			}
			if !inserted {
				if idx := strings.Index(trimmed, " redirect="); idx >= 0 {
					res.Value = trimmed[:idx] + insert + trimmed[idx:]
					inserted = true
				}
			}
			if !inserted {
				res.Value = trimmed + insert + " -all"
			}
			res.Changed = true
		}
	}

	if mo.resolver != nil {
		total, rerr := recursiveSPFLookupCount(context.Background(), mo.resolver, res.Value, 0, make(map[string]bool))
		if rerr != nil {
			res.Warnings = append(res.Warnings, "recursive SPF lookup count incomplete: "+rerr.Error())
			// Fall back to surface count so callers always get a number.
			res.LookupCount = countSPFLookups(res.Value)
		} else {
			res.LookupCount = total
		}
	} else {
		res.LookupCount = countSPFLookups(res.Value)
	}
	if res.LookupCount > 10 {
		res.LookupLimitExceeded = true
		res.Warnings = append(res.Warnings, "SPF record exceeds 10 DNS lookup limit (RFC 7208); mail receivers may return permerror")
	}
	return res, nil
}

// recursiveSPFLookupCount walks include:/redirect= mechanisms up to RFC 7208's
// depth-10 limit and sums every DNS-mechanism count along the way. a:, mx:,
// ptr:, exists: at the top level count as 1 each but are not themselves
// followed. include: and redirect= each count as 1 at the current level AND
// have their targets fetched and counted recursively. Cycles and already-seen
// domains are short-circuited via seen. A nil resolver is a programming error
// and should not reach here.
func recursiveSPFLookupCount(ctx context.Context, r SPFResolver, spf string, depth int, seen map[string]bool) (int, error) {
	if depth > 10 {
		return 0, nil // RFC 7208 cap; caller-surfaced via LookupLimitExceeded downstream if total>10.
	}
	count := 0
	for _, tok := range strings.Fields(spf) {
		low := strings.ToLower(tok)
		switch {
		case low == "a", low == "mx":
			count++
		case strings.HasPrefix(low, "a:"),
			strings.HasPrefix(low, "mx:"),
			strings.HasPrefix(low, "ptr:"),
			strings.HasPrefix(low, "exists:"):
			count++
		case strings.HasPrefix(low, "include:"):
			count++
			target := strings.TrimPrefix(tok, "include:")
			// Mechanisms may have +/-/~/? qualifier prefixes (e.g. "-include:").
			target = stripQualifier(target)
			sub, err := recurseInto(ctx, r, target, depth, seen)
			if err != nil {
				return count, err
			}
			count += sub
		case strings.HasPrefix(low, "redirect="):
			count++
			target := strings.TrimPrefix(tok, "redirect=")
			sub, err := recurseInto(ctx, r, target, depth, seen)
			if err != nil {
				return count, err
			}
			count += sub
		}
	}
	return count, nil
}

func recurseInto(ctx context.Context, r SPFResolver, target string, depth int, seen map[string]bool) (int, error) {
	target = strings.TrimSuffix(target, ".")
	if target == "" || seen[target] {
		return 0, nil
	}
	seen[target] = true
	txts, err := r.LookupTXT(ctx, target)
	if err != nil {
		return 0, err
	}
	for _, txt := range txts {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(txt)), "v=spf1") {
			continue
		}
		return recursiveSPFLookupCount(ctx, r, txt, depth+1, seen)
	}
	return 0, nil
}

func stripQualifier(mech string) string {
	// Handles cases like "-include:foo" that arrive after the prefix trim.
	// Our callers pass post-prefix targets, but be defensive.
	if len(mech) == 0 {
		return mech
	}
	switch mech[0] {
	case '+', '-', '~', '?':
		return mech[1:]
	}
	return mech
}

func countSPFLookups(spf string) int {
	count := 0
	for _, tok := range strings.Fields(spf) {
		low := strings.ToLower(tok)
		switch {
		case low == "a", low == "mx":
			count++
		case strings.HasPrefix(low, "include:"),
			strings.HasPrefix(low, "a:"),
			strings.HasPrefix(low, "mx:"),
			strings.HasPrefix(low, "ptr:"),
			strings.HasPrefix(low, "exists:"),
			strings.HasPrefix(low, "redirect="):
			count++
		}
	}
	return count
}
