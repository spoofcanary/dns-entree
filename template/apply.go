// Package template apply.go: ApplyTemplate end-to-end orchestration.
//
// Resolves a Template, handles TXT conflict modes (D-16/D-17), dispatches each
// record through PushService (D-19), and returns one PushResult per record
// with partial-failure semantics (D-20). SPFM is auto-routed to PushSPFRecord
// (D-26).
package template

import (
	"context"
	"errors"
	"fmt"
	"strings"

	entree "github.com/spoofcanary/dns-entree"
)

// ApplyTemplate resolves tmpl with vars and pushes every record at domain via
// pushSvc. Conflict modes Prefix/Exact/All trigger pre-push deletes against
// existing TXT records at the same Name. Mode None (or empty) skips conflict
// resolution. Errors from individual records are collected and returned as a
// joined error; processing continues so callers always receive a PushResult
// per record.
func ApplyTemplate(
	ctx context.Context,
	pushSvc *entree.PushService,
	domain string,
	tmpl *Template,
	vars map[string]string,
) ([]*entree.PushResult, error) {
	resolved, err := tmpl.ResolveDetailed(vars)
	if err != nil {
		return nil, err
	}

	results := make([]*entree.PushResult, 0, len(resolved))
	var errs []error

	for _, rr := range resolved {
		// Conflict resolution (TXT only, per spec).
		if rr.Record.Type == "TXT" && rr.Mode != "" && rr.Mode != "None" {
			if cerr := applyConflicts(ctx, pushSvc, domain, rr); cerr != nil {
				results = append(results, &entree.PushResult{
					Status:      entree.StatusFailed,
					RecordName:  rr.Record.Name,
					RecordValue: rr.Record.Content,
				})
				errs = append(errs, fmt.Errorf("conflict resolution for %s: %w", rr.Record.Name, cerr))
				continue
			}
		}

		res, perr := dispatchRecord(ctx, pushSvc, domain, rr.Record)
		if res == nil {
			res = &entree.PushResult{
				Status:      entree.StatusFailed,
				RecordName:  rr.Record.Name,
				RecordValue: rr.Record.Content,
			}
		}
		results = append(results, res)
		if perr != nil {
			errs = append(errs, fmt.Errorf("push %s %s: %w", rr.Record.Type, rr.Record.Name, perr))
		}
	}

	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}

// dispatchRecord routes a single resolved record to the correct PushService
// method based on its Type.
func dispatchRecord(
	ctx context.Context,
	pushSvc *entree.PushService,
	domain string,
	rec entree.Record,
) (*entree.PushResult, error) {
	switch rec.Type {
	case "TXT":
		return pushSvc.PushTXTRecord(ctx, domain, rec.Name, rec.Content)
	case "CNAME":
		return pushSvc.PushCNAMERecord(ctx, domain, rec.Name, rec.Content)
	case "SPFM":
		include := extractInclude(rec.Content)
		if include == "" {
			// Some official templates ship SPFM with no data; the include is
			// encoded elsewhere or simply absent. Skip rather than fail.
			return &entree.PushResult{
				Status:      entree.StatusAlreadyConfigured,
				RecordName:  rec.Name,
				RecordValue: "",
			}, nil
		}
		return pushSvc.PushSPFRecord(ctx, domain, []string{include})
	case "A", "AAAA", "MX", "NS", "SRV":
		return pushSvc.PushGenericRecord(ctx, domain, rec)
	default:
		return &entree.PushResult{
			Status:      entree.StatusFailed,
			RecordName:  rec.Name,
			RecordValue: rec.Content,
		}, fmt.Errorf("unsupported record type: %s", rec.Type)
	}
}

// applyConflicts deletes existing TXT records at rr.Record.Name that match the
// configured conflict mode. Only TXT records at the same Name are touched
// (T-04-11 mitigation).
func applyConflicts(
	ctx context.Context,
	pushSvc *entree.PushService,
	domain string,
	rr ResolvedRecord,
) error {
	prov := pushSvc.Provider()
	existing, err := prov.GetRecords(ctx, domain, "TXT")
	if err != nil {
		return fmt.Errorf("get existing TXT: %w", err)
	}
	for _, e := range existing {
		if e.Name != rr.Record.Name {
			continue
		}
		match := false
		switch rr.Mode {
		case "Prefix":
			if rr.Prefix != "" && strings.HasPrefix(e.Content, rr.Prefix) {
				match = true
			}
		case "Exact":
			if e.Content == rr.Record.Content {
				match = true
			}
		case "All":
			match = true
		}
		if !match {
			continue
		}
		if err := prov.DeleteRecord(ctx, domain, e.ID); err != nil {
			return fmt.Errorf("delete %s: %w", e.ID, err)
		}
	}
	return nil
}

// extractInclude pulls the include target out of an SPF data string. Per D-26,
// the SPFM record's Data field carries the include target, but template
// authors may write either "v=spf1 include:foo.com ~all" or just "foo.com" or
// "include:foo.com".
func extractInclude(spfData string) string {
	s := strings.TrimSpace(spfData)
	if s == "" {
		return ""
	}
	// If it looks like a full SPF record, find the first include: token.
	if idx := strings.Index(s, "include:"); idx >= 0 {
		rest := s[idx+len("include:"):]
		// terminate at whitespace
		if sp := strings.IndexAny(rest, " \t"); sp >= 0 {
			rest = rest[:sp]
		}
		return rest
	}
	return s
}
