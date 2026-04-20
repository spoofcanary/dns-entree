package esp

import (
	"context"
	"strings"

	entree "github.com/spoofcanary/dns-entree"
)

// ClassifyFromSPF classifies senders based solely on the top-level include
// tokens of an SPF record. It does not perform DNS lookups. Returns one
// classification per recognized include; unknown includes are skipped.
//
// Use ClassifyFromSPFRecursive when you need SES-in-chain detection
// (e.g., to flag a domain using Resend as ses-backed because
// _spf.resend.com expands into amazonses.com).
func ClassifyFromSPF(spfRecord string) []SenderClassification {
	includes := extractIncludes(spfRecord)
	out := make([]SenderClassification, 0, len(includes))
	for _, inc := range includes {
		if info, ok := LookupByInclude(inc); ok {
			out = append(out, SenderClassification{
				Name:           info.Name,
				Category:       info.Category,
				Infrastructure: info.Infrastructure,
				Integration:    info.Integration,
				SPFSource:      inc,
			})
		}
	}
	return out
}

// ClassifyFromSPFRecursive is ClassifyFromSPF + recursive include walk.
// For each top-level include, it walks the SPF chain (up to RFC 7208 depth
// 10) and flags Infrastructure=InfraSES when amazonses.com appears anywhere
// below. This catches ESPs like Resend, Loops, Bento, Customer.io and
// Salesforce transactional that customers use without knowing they run on
// SES.
//
// The classification for a known top-level include uses the catalog entry
// verbatim. For unknown top-level includes, a "Custom sender" entry is
// emitted with whatever infrastructure was detected via chain walk.
//
// Requires a resolver (see entree.NewNetSPFResolver).
func ClassifyFromSPFRecursive(ctx context.Context, resolver entree.SPFResolver, spfRecord string) []SenderClassification {
	if resolver == nil {
		return ClassifyFromSPF(spfRecord)
	}
	includes := extractIncludes(spfRecord)
	out := make([]SenderClassification, 0, len(includes))
	for _, inc := range includes {
		classification := SenderClassification{SPFSource: inc}

		if info, ok := LookupByInclude(inc); ok {
			classification.Name = info.Name
			classification.Category = info.Category
			classification.Infrastructure = info.Infrastructure
			classification.Integration = info.Integration
		} else {
			classification.Name = "Custom sender (" + inc + ")"
			classification.Category = CategoryUnknown
			classification.Infrastructure = InfraUnknown
			classification.Integration = IntegrationManual
		}

		// Walk chain only if we haven't already classified as SES. If the
		// catalog already says SES, no need to confirm.
		if classification.Infrastructure == InfraUnknown ||
			classification.Infrastructure == InfraSESMixed {
			if isSESInChain(ctx, resolver, inc) {
				classification.ViaChain = true
				if classification.Infrastructure == InfraUnknown {
					classification.Infrastructure = InfraSES
				}
				// ses_mixed stays ses_mixed - we already knew it was partly SES.
			}
		}

		out = append(out, classification)
	}
	return out
}

// isSESInChain walks an SPF include target's chain looking for amazonses.com.
// Returns true if found anywhere up to depth 10. Cycles short-circuited.
func isSESInChain(ctx context.Context, r entree.SPFResolver, target string) bool {
	target = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(target)), ".")
	if target == "amazonses.com" {
		return true
	}
	seen := make(map[string]bool)
	return walkForSES(ctx, r, target, 0, seen)
}

func walkForSES(ctx context.Context, r entree.SPFResolver, target string, depth int, seen map[string]bool) bool {
	if depth > 10 || target == "" || seen[target] {
		return false
	}
	seen[target] = true
	if target == "amazonses.com" {
		return true
	}
	txts, err := r.LookupTXT(ctx, target)
	if err != nil {
		return false
	}
	for _, txt := range txts {
		low := strings.ToLower(strings.TrimSpace(txt))
		if !strings.HasPrefix(low, "v=spf1") {
			continue
		}
		for _, tok := range strings.Fields(low) {
			switch {
			case strings.HasPrefix(tok, "include:"):
				sub := stripSPFQualifier(strings.TrimPrefix(tok, "include:"))
				sub = strings.TrimSuffix(sub, ".")
				if sub == "amazonses.com" {
					return true
				}
				if walkForSES(ctx, r, sub, depth+1, seen) {
					return true
				}
			case strings.HasPrefix(tok, "redirect="):
				sub := strings.TrimPrefix(tok, "redirect=")
				sub = strings.TrimSuffix(sub, ".")
				if sub == "amazonses.com" {
					return true
				}
				if walkForSES(ctx, r, sub, depth+1, seen) {
					return true
				}
			}
		}
	}
	return false
}

// extractIncludes pulls bare include: target hosts out of an SPF record.
// Handles qualifier prefixes (+/-/~/?) and lowercases output.
func extractIncludes(spfRecord string) []string {
	out := make([]string, 0, 8)
	for _, tok := range strings.Fields(spfRecord) {
		low := strings.ToLower(tok)
		if !strings.HasPrefix(low, "include:") {
			// Qualifier-prefixed include ("-include:foo") rare but valid.
			if len(low) >= 9 {
				switch low[0] {
				case '+', '-', '~', '?':
					if strings.HasPrefix(low[1:], "include:") {
						out = append(out, strings.TrimSuffix(low[len("include:")+1:], "."))
					}
				}
			}
			continue
		}
		target := strings.TrimPrefix(low, "include:")
		target = strings.TrimSuffix(target, ".")
		if target != "" {
			out = append(out, target)
		}
	}
	return out
}

func stripSPFQualifier(mech string) string {
	if len(mech) == 0 {
		return mech
	}
	switch mech[0] {
	case '+', '-', '~', '?':
		return mech[1:]
	}
	return mech
}
