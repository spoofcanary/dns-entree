package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	entree "github.com/spoofcanary/dns-entree"
)

// MigrateOptions configures a Migrate call.
type MigrateOptions struct {
	Domain         string
	TargetSlug     string
	TargetProvider entree.Provider
	ProviderOpts   ProviderOpts
	ScrapeOpts     ScrapeOptions
	SourceBindFile string
	Apply          bool
	RatePerSecond  float64
	VerifyTimeout  time.Duration
	QueryTimeout   time.Duration
	// SourceProviderSlug overrides automatic source registrar detection.
	SourceProviderSlug string
	// SkipSourceDetect disables the DetectProvider network call (used by tests).
	SkipSourceDetect bool
}

// Migrate runs the two-phase migration: Phase A (scrape + preview + ensure
// zone) and, when opts.Apply is true, Phase B (rate-limited bulk apply +
// post-apply verification against the assigned target nameservers). The
// returned report is always non-nil; error is non-nil when apply errors or
// mismatches occurred.
func Migrate(ctx context.Context, opts MigrateOptions) (*MigrationReport, error) {
	if err := validateDomain(opts.Domain); err != nil {
		return &MigrationReport{Domain: opts.Domain}, err
	}
	if opts.TargetSlug == "" {
		return &MigrationReport{Domain: opts.Domain}, fmt.Errorf("migrate: TargetSlug required")
	}
	if opts.TargetProvider == nil {
		return &MigrationReport{Domain: opts.Domain}, fmt.Errorf("migrate: TargetProvider required")
	}
	if opts.VerifyTimeout <= 0 {
		opts.VerifyTimeout = 5 * time.Minute
	}
	if opts.QueryTimeout <= 0 {
		opts.QueryTimeout = 5 * time.Second
	}

	report := &MigrationReport{Domain: opts.Domain}

	// ------- Phase A: acquire source zone.
	var zone *Zone
	var err error
	if opts.SourceBindFile != "" {
		zone, err = ImportBINDFile(opts.SourceBindFile, opts.Domain)
	} else {
		zone, err = ScrapeZone(ctx, opts.Domain, opts.ScrapeOpts)
	}
	if err != nil {
		report.Errors = append(report.Errors, fmt.Errorf("scrape: %w", err))
		return report, errors.Join(report.Errors...)
	}
	report.Source = zone.Source
	report.SourceNameservers = zone.Nameservers
	report.Preview = zone.Records
	report.Warnings = append(report.Warnings, zone.Warnings...)

	// ------- Phase A: resolve target adapter + ensure zone.
	adapter, err := GetAdapter(opts.TargetSlug)
	if err != nil {
		report.Errors = append(report.Errors, err)
		report.TargetZoneStatus = "error"
		return report, errors.Join(report.Errors...)
	}

	zi, err := adapter.EnsureZone(ctx, opts.Domain, opts.ProviderOpts)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Errorf("ensure zone: %w", err))
		report.TargetZoneStatus = "error"
		return report, errors.Join(report.Errors...)
	}
	report.TargetZone = zi
	if zi.Created {
		report.TargetZoneStatus = "will_create"
	} else {
		report.TargetZoneStatus = "exists"
	}

	// ------- Source provider detection for NS instructions.
	sourceSlug := opts.SourceProviderSlug
	if sourceSlug == "" && !opts.SkipSourceDetect {
		if det, derr := entree.DetectProvider(ctx, opts.Domain); derr == nil && det != nil {
			sourceSlug = string(det.Provider)
		}
	}
	report.SourceProvider = sourceSlug
	report.NSChange = FormatNSChangeInstructions(sourceSlug, zi.Nameservers)

	// Phase A complete. Dry-run stops here (D-18).
	if !opts.Apply {
		return report, nil
	}

	// ------- Phase B: rate-limited apply.
	limiter := NewWriteLimiter(opts.RatePerSecond)
	applied := make([]entree.Record, 0, len(zone.Records))
	for _, rec := range zone.Records {
		if err := limiter.Wait(ctx); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("rate limit wait: %w", err))
			return report, errors.Join(report.Errors...)
		}
		if err := opts.TargetProvider.SetRecord(ctx, opts.Domain, rec); err != nil {
			report.Results = append(report.Results, RecordResult{
				Record: rec,
				Status: StatusApplyFailed,
				Detail: sanitizeErr(err),
			})
			report.Errors = append(report.Errors, fmt.Errorf("apply %s %s: %w", rec.Type, rec.Name, err))
			continue
		}
		applied = append(applied, rec)
	}

	// ------- Phase B: verify against target nameservers (D-22/D-23).
	deadline := time.Now().Add(opts.VerifyTimeout)
	pending := applied
	var finalResults []RecordResult
	for {
		var still []entree.Record
		for _, rec := range pending {
			matched, gotAny, detail := verifyRecordAgainstNS(ctx, zi.Nameservers, rec, opts.QueryTimeout)
			if matched {
				finalResults = append(finalResults, RecordResult{Record: rec, Status: StatusMatched, Detail: detail})
				continue
			}
			if !gotAny {
				still = append(still, rec)
				continue
			}
			finalResults = append(finalResults, RecordResult{Record: rec, Status: StatusMismatch, Detail: detail})
		}
		if len(still) == 0 || time.Now().After(deadline) {
			for _, rec := range still {
				finalResults = append(finalResults, RecordResult{
					Record: rec,
					Status: StatusNotYetPropagated,
					Detail: "no answer from target nameservers within timeout",
				})
			}
			break
		}
		pending = still
		select {
		case <-ctx.Done():
			for _, rec := range pending {
				finalResults = append(finalResults, RecordResult{Record: rec, Status: StatusNotYetPropagated, Detail: "context cancelled"})
			}
			report.Results = append(report.Results, finalResults...)
			report.Errors = append(report.Errors, ctx.Err())
			return report, errors.Join(report.Errors...)
		case <-time.After(2 * time.Second):
		}
	}
	report.Results = append(report.Results, finalResults...)

	for _, r := range report.Results {
		if r.Status == StatusMismatch {
			report.Errors = append(report.Errors, fmt.Errorf("mismatch: %s %s: %s", r.Record.Type, r.Record.Name, r.Detail))
		}
	}

	if len(report.Errors) > 0 {
		return report, errors.Join(report.Errors...)
	}
	return report, nil
}

// sanitizeErr strips potential credential material from error strings before
// surfacing to reports (T-05b-09).
func sanitizeErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	for _, needle := range []string{"Authorization:", "Bearer ", "api_key", "apikey", "X-Api-Key"} {
		if i := strings.Index(strings.ToLower(s), strings.ToLower(needle)); i >= 0 {
			return s[:i] + "[redacted]"
		}
	}
	return s
}
