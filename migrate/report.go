package migrate

import (
	entree "github.com/spoofcanary/dns-entree"
)

// Status classifies a single record's post-apply verification outcome.
type Status int

const (
	StatusMatched Status = iota
	StatusMismatch
	StatusNotYetPropagated
	StatusApplyFailed
)

func (s Status) String() string {
	switch s {
	case StatusMatched:
		return "matched"
	case StatusMismatch:
		return "mismatch"
	case StatusNotYetPropagated:
		return "not_yet_propagated"
	case StatusApplyFailed:
		return "apply_failed"
	}
	return "unknown"
}

// RecordResult is the per-record outcome of apply + verify.
type RecordResult struct {
	Record entree.Record
	Status Status
	Detail string
}

// MigrationReport is the full result of a Migrate call. It is always returned
// even when an error is non-nil so callers can render partial progress.
type MigrationReport struct {
	Domain            string
	Source            string // "axfr" | "iterated" | "bind"
	SourceNameservers []string
	SourceProvider    string
	TargetZone        ZoneInfo
	TargetZoneStatus  string // "will_create" | "exists" | "error"
	Preview           []entree.Record
	Results           []RecordResult
	Warnings          []string
	Errors            []error
	NSChange          string
}
