package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/migrate"
)

// Flags shared by zone export/import and migrate.
var (
	flagZoneOutput     string
	flagZoneFormat     string
	flagZoneLabels     []string
	flagZoneLabelsOnly []string
	flagZoneLabelsFile string
	flagZoneNoAXFR     bool

	flagZoneImportFrom    string
	flagZoneImportTo      string
	flagZoneRate          float64
	flagZoneDryRun        bool
	flagZoneVerifyTimeout time.Duration
)

// zoneExportEnvelope is the schema_version:1 data payload for `zone export`.
type zoneExportData struct {
	SchemaVersion int             `json:"schema_version"`
	Command       string          `json:"command"`
	Domain        string          `json:"domain"`
	Source        string          `json:"source"`
	Nameservers   []string        `json:"nameservers"`
	Records       []entree.Record `json:"records"`
	Warnings      []string        `json:"warnings,omitempty"`
}

var zoneCmd = &cobra.Command{
	Use:   "zone",
	Short: "Zone export/import for cross-provider migration",
}

var zoneExportCmd = &cobra.Command{
	Use:   "export <domain>",
	Short: "Scrape a DNS zone and emit JSON or BIND",
	Args:  cobra.ExactArgs(1),
	RunE:  runZoneExport,
}

var zoneImportCmd = &cobra.Command{
	Use:   "import <domain>",
	Short: "Apply a previously-exported zone to a target provider",
	Args:  cobra.ExactArgs(1),
	RunE:  runZoneImport,
}

func runZoneExport(cmd *cobra.Command, args []string) error {
	domain := strings.TrimSpace(args[0])
	if domain == "" {
		return &UserError{Code: "INVALID_DOMAIN", Msg: "domain required"}
	}
	format := strings.ToLower(flagZoneFormat)
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "bind" {
		return &UserError{Code: "INVALID_FORMAT", Msg: "--format must be json or bind"}
	}

	labels, err := loadLabelsFile(flagZoneLabelsFile)
	if err != nil {
		return &UserError{Code: "LABELS_FILE_READ_FAILED", Msg: err.Error()}
	}
	extra := append([]string{}, flagZoneLabels...)
	extra = append(extra, labels...)

	ctx, cancel := context.WithTimeout(cmd.Context(), flagTimeout)
	defer cancel()

	zone, err := migrate.ScrapeZone(ctx, domain, migrate.ScrapeOptions{
		ExtraLabels: extra,
		OnlyLabels:  flagZoneLabelsOnly,
		SkipAXFR:    flagZoneNoAXFR,
	})
	if err != nil {
		return &RuntimeError{Code: "SCRAPE_FAILED", Msg: err.Error()}
	}

	data := zoneExportData{
		SchemaVersion: SchemaVersion,
		Command:       "zone.export",
		Domain:        zone.Domain,
		Source:        zone.Source,
		Nameservers:   zone.Nameservers,
		Records:       zone.Records,
		Warnings:      zone.Warnings,
	}

	var body []byte
	if format == "bind" {
		body = []byte(renderBIND(zone))
	} else {
		// When --json global flag is set, we emit via formatter envelope.
		// Otherwise we emit the raw data payload as JSON.
		f := formatterFromCtx(cmd.Context())
		if f.Mode == ModeJSON {
			if flagZoneOutput != "" {
				buf, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					return &RuntimeError{Code: "JSON_ENCODE_FAILED", Msg: err.Error()}
				}
				return writeOutput(flagZoneOutput, append(buf, '\n'))
			}
			return f.EmitOK(data)
		}
		buf, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return &RuntimeError{Code: "JSON_ENCODE_FAILED", Msg: err.Error()}
		}
		body = append(buf, '\n')
	}

	if flagZoneOutput != "" {
		return writeOutput(flagZoneOutput, body)
	}
	_, err = os.Stdout.Write(body)
	return err
}

func runZoneImport(cmd *cobra.Command, args []string) error {
	domain := strings.TrimSpace(args[0])
	if domain == "" {
		return &UserError{Code: "INVALID_DOMAIN", Msg: "domain required"}
	}
	if flagZoneImportFrom == "" {
		return &UserError{Code: "MISSING_FROM", Msg: "--from <file> required"}
	}
	if flagZoneImportTo == "" {
		return &UserError{Code: "MISSING_TO", Msg: "--to <slug> required"}
	}

	zone, err := loadZoneFile(flagZoneImportFrom, domain)
	if err != nil {
		return &UserError{Code: "ZONE_LOAD_FAILED", Msg: err.Error()}
	}

	return runMigrateCore(cmd, domain, flagZoneImportTo, migrateCoreOpts{
		zone:          zone,
		dryRun:        flagZoneDryRun,
		rate:          flagZoneRate,
		skipAXFR:      true,
		verifyTimeout: flagZoneVerifyTimeout,
	})
}

// ----------------- helpers -----------------

func writeOutput(path string, body []byte) error {
	return os.WriteFile(path, body, 0o600)
}

func loadLabelsFile(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

func renderBIND(z *migrate.Zone) string {
	var b strings.Builder
	fmt.Fprintf(&b, "; dns-entree zone export for %s\n", z.Domain)
	fmt.Fprintf(&b, "; source=%s\n", z.Source)
	fmt.Fprintf(&b, "$ORIGIN %s.\n", z.Domain)
	for _, r := range z.Records {
		name := r.Name
		if name == z.Domain || name == "" {
			name = "@"
		} else if strings.HasSuffix(name, "."+z.Domain) {
			name = strings.TrimSuffix(name, "."+z.Domain)
		}
		ttl := r.TTL
		if ttl <= 0 {
			ttl = 300
		}
		switch r.Type {
		case "TXT":
			// Escape backslashes FIRST, then quotes. Reversing this order
			// leaves literal backslashes inconsistently escaped and produces
			// invalid BIND output.
			content := strings.ReplaceAll(r.Content, `\`, `\\`)
			content = strings.ReplaceAll(content, `"`, `\"`)
			fmt.Fprintf(&b, "%s\t%d\tIN\tTXT\t\"%s\"\n", name, ttl, content)
		case "MX":
			fmt.Fprintf(&b, "%s\t%d\tIN\tMX\t%d %s\n", name, ttl, r.Priority, ensureDot(r.Content))
		case "CNAME", "NS":
			fmt.Fprintf(&b, "%s\t%d\tIN\t%s\t%s\n", name, ttl, r.Type, ensureDot(r.Content))
		case "SRV":
			fmt.Fprintf(&b, "%s\t%d\tIN\tSRV\t%d %d %d %s\n", name, ttl, r.Priority, r.Weight, r.Port, ensureDot(r.Content))
		default:
			fmt.Fprintf(&b, "%s\t%d\tIN\t%s\t%s\n", name, ttl, r.Type, r.Content)
		}
	}
	return b.String()
}

func ensureDot(s string) string {
	if s == "" || strings.HasSuffix(s, ".") {
		return s
	}
	return s + "."
}

// loadZoneFile detects JSON (schema_version:1) vs BIND and parses accordingly.
func loadZoneFile(path, domain string) (*migrate.Zone, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return parseZoneJSON(trimmed, domain)
	}
	return migrate.ImportBIND(bytes.NewReader(data), domain)
}

func parseZoneJSON(data []byte, domain string) (*migrate.Zone, error) {
	// Accept either raw payload {schema_version, command, domain, records, ...}
	// or outer formatter envelope {ok, schema_version, data:{...}}.
	var env struct {
		OK            *bool           `json:"ok"`
		SchemaVersion int             `json:"schema_version"`
		Data          json.RawMessage `json:"data"`
	}
	payload := data
	if err := json.Unmarshal(data, &env); err == nil && env.OK != nil && len(env.Data) > 0 {
		payload = env.Data
	}
	var doc zoneExportData
	if err := json.Unmarshal(payload, &doc); err != nil {
		return nil, fmt.Errorf("parse zone json: %w", err)
	}
	if doc.SchemaVersion != 0 && doc.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported zone schema_version %d", doc.SchemaVersion)
	}
	d := doc.Domain
	if d == "" {
		d = domain
	}
	return &migrate.Zone{
		Domain:      d,
		Records:     doc.Records,
		Source:      firstNonEmpty(doc.Source, "json"),
		Nameservers: doc.Nameservers,
		Warnings:    doc.Warnings,
	}, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// ----------------- shared migrate core -----------------

type migrateCoreOpts struct {
	zone          *migrate.Zone // pre-loaded (import path); nil means Migrate scrapes
	dryRun        bool
	rate          float64
	skipAXFR      bool
	scrapeLabels  []string
	scrapeOnly    []string
	verifyTimeout time.Duration
}

// runMigrateCore resolves creds, builds ProviderOpts, invokes migrate.Migrate,
// and emits the report. Used by both `zone import` and `migrate`.
func runMigrateCore(cmd *cobra.Command, domain, slug string, opts migrateCoreOpts) error {
	if slug == "" {
		return &UserError{Code: "MISSING_TO", Msg: "--to <slug> required"}
	}

	apply := flagYes && !opts.dryRun
	if apply {
		if err := RequireYes(IsTTY(os.Stdout), flagYes); err != nil {
			return err
		}
	}

	creds, err := NewCredentialsLoader(flagCredentialsFile).Load(slug)
	if err != nil {
		return err
	}
	provider, err := entree.NewProvider(slug, creds)
	if err != nil {
		return &RuntimeError{Code: "PROVIDER_INIT_FAILED", Msg: err.Error()}
	}

	providerOpts := migrate.ProviderOpts{
		APIToken:  creds.APIToken,
		APIKey:    creds.APIKey,
		APISecret: creds.APISecret,
		AccessKey: creds.AccessKey,
		SecretKey: creds.SecretKey,
		Region:    creds.Region,
		Token:     creds.Token,
		ProjectID: creds.ProjectID,
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 2*flagTimeout+opts.verifyTimeout)
	defer cancel()

	mopts := migrate.MigrateOptions{
		Domain:           domain,
		TargetSlug:       slug,
		TargetProvider:   provider,
		ProviderOpts:     providerOpts,
		Apply:            apply,
		RatePerSecond:    opts.rate,
		VerifyTimeout:    opts.verifyTimeout,
		SkipSourceDetect: true,
		ScrapeOpts: migrate.ScrapeOptions{
			SkipAXFR:    opts.skipAXFR,
			ExtraLabels: opts.scrapeLabels,
			OnlyLabels:  opts.scrapeOnly,
		},
	}

	// If caller pre-loaded a zone (import path), pass it directly to Migrate
	// to avoid a lossy BIND round-trip through a temp file.
	if opts.zone != nil {
		mopts.PreloadedZone = opts.zone
	}

	report, runErr := migrate.Migrate(ctx, mopts)

	f := formatterFromCtx(cmd.Context())
	payload := reportToJSON(report, opts.dryRun || !apply)
	if f.Mode == ModeJSON {
		if emitErr := f.EmitOK(payload); emitErr != nil {
			return emitErr
		}
	} else {
		renderReportHuman(f, report, opts.dryRun || !apply)
	}

	if runErr != nil {
		// Classify: mismatch vs apply failure.
		if hasMismatch(report) {
			return &RuntimeError{Code: "VERIFY_MISMATCH", Msg: "post-apply verification mismatch"}
		}
		return &RuntimeError{Code: "MIGRATE_PARTIAL_FAILURE", Msg: sanitizeString(runErr.Error())}
	}
	return nil
}

type migrateReportJSON struct {
	SchemaVersion    int             `json:"schema_version"`
	Command          string          `json:"command"`
	Domain           string          `json:"domain"`
	Applied          bool            `json:"applied"`
	Source           string          `json:"source"`
	SourceProvider   string          `json:"source_provider,omitempty"`
	TargetZoneStatus string          `json:"target_zone_status"`
	TargetZone       migrateZoneJSON `json:"target_zone"`
	Preview          []entree.Record `json:"preview"`
	Results          []resultJSON    `json:"results,omitempty"`
	Warnings         []string        `json:"warnings,omitempty"`
	Errors           []string        `json:"errors,omitempty"`
	NSChange         string          `json:"ns_change,omitempty"`
}

type migrateZoneJSON struct {
	ZoneID      string   `json:"zone_id,omitempty"`
	Nameservers []string `json:"nameservers,omitempty"`
	Created     bool     `json:"created"`
}

type resultJSON struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

func reportToJSON(r *migrate.MigrationReport, previewOnly bool) migrateReportJSON {
	out := migrateReportJSON{
		SchemaVersion:    SchemaVersion,
		Command:          "migrate",
		Domain:           r.Domain,
		Applied:          !previewOnly,
		Source:           r.Source,
		SourceProvider:   r.SourceProvider,
		TargetZoneStatus: r.TargetZoneStatus,
		TargetZone: migrateZoneJSON{
			ZoneID:      r.TargetZone.ZoneID,
			Nameservers: r.TargetZone.Nameservers,
			Created:     r.TargetZone.Created,
		},
		Preview:  r.Preview,
		Warnings: r.Warnings,
		NSChange: r.NSChange,
	}
	for _, res := range r.Results {
		out.Results = append(out.Results, resultJSON{
			Type:    res.Record.Type,
			Name:    res.Record.Name,
			Content: res.Record.Content,
			Status:  res.Status.String(),
			Detail:  sanitizeString(res.Detail),
		})
	}
	for _, e := range r.Errors {
		if e == nil {
			continue
		}
		out.Errors = append(out.Errors, sanitizeString(e.Error()))
	}
	return out
}

func renderReportHuman(f *Formatter, r *migrate.MigrationReport, previewOnly bool) {
	fmt.Fprintf(f.Out, "domain: %s\n", r.Domain)
	fmt.Fprintf(f.Out, "source: %s\n", r.Source)
	fmt.Fprintf(f.Out, "target_zone_status: %s\n", r.TargetZoneStatus)
	fmt.Fprintf(f.Out, "preview_records: %d\n", len(r.Preview))
	if previewOnly {
		fmt.Fprintln(f.Out, "mode: dry-run (pass --yes to apply)")
	} else {
		fmt.Fprintf(f.Out, "results: %d\n", len(r.Results))
	}
	if r.NSChange != "" {
		fmt.Fprintln(f.Err, r.NSChange)
	}
}

func hasMismatch(r *migrate.MigrationReport) bool {
	for _, res := range r.Results {
		if res.Status == migrate.StatusMismatch {
			return true
		}
	}
	return false
}

// sanitizeString strips anything that looks like a credential before surfacing.
func sanitizeString(s string) string {
	for _, needle := range []string{"Bearer ", "Authorization:", "api_token", "api_key", "api_secret"} {
		if i := strings.Index(strings.ToLower(s), strings.ToLower(needle)); i >= 0 {
			return s[:i] + "[redacted]"
		}
	}
	return s
}

func init() {
	// zone export flags
	zoneExportCmd.Flags().StringVar(&flagZoneOutput, "output", "", "write output to file instead of stdout")
	zoneExportCmd.Flags().StringVar(&flagZoneFormat, "format", "json", "output format: json|bind")
	zoneExportCmd.Flags().StringSliceVar(&flagZoneLabels, "labels", nil, "additional labels to enumerate (comma-separated, additive)")
	zoneExportCmd.Flags().StringSliceVar(&flagZoneLabelsOnly, "labels-only", nil, "replace default label list with this set")
	zoneExportCmd.Flags().StringVar(&flagZoneLabelsFile, "labels-file", "", "file with extra labels, one per line")
	zoneExportCmd.Flags().BoolVar(&flagZoneNoAXFR, "no-axfr", false, "disable AXFR attempt")

	// zone import flags
	zoneImportCmd.Flags().StringVar(&flagZoneImportFrom, "from", "", "path to exported zone JSON or BIND file (required)")
	zoneImportCmd.Flags().StringVar(&flagZoneImportTo, "to", "", "target provider slug (required)")
	zoneImportCmd.Flags().BoolVar(&flagZoneDryRun, "dry-run", false, "preview without applying")
	zoneImportCmd.Flags().Float64Var(&flagZoneRate, "rate", 10, "writes per second rate limit (max 100)")
	zoneImportCmd.Flags().DurationVar(&flagZoneVerifyTimeout, "verify-timeout", 5*time.Second, "post-apply verification timeout")

	zoneCmd.AddCommand(zoneExportCmd)
	zoneCmd.AddCommand(zoneImportCmd)
	rootCmd.AddCommand(zoneCmd)
}
