package entree

import "strings"

// MergeResult is the result of merging includes into an SPF record.
type MergeResult struct {
	Value               string
	Changed             bool
	BrokenInput         bool
	LookupCount         int
	LookupLimitExceeded bool
	Warnings            []string
}

// MergeSPF merges the given include domains into current, preserving all
// existing mechanisms. Idempotent: re-running on its own output with the same
// includes returns Changed=false. Caller is responsible for validating include
// strings -- they are passed through verbatim.
func MergeSPF(current string, includes []string) (MergeResult, error) {
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

	res.LookupCount = countSPFLookups(res.Value)
	if res.LookupCount > 10 {
		res.LookupLimitExceeded = true
		res.Warnings = append(res.Warnings, "SPF record exceeds 10 DNS lookup limit (RFC 7208); mail receivers may return permerror")
	}
	return res, nil
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
