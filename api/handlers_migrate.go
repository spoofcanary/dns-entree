package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/migrate"
)

// bodyCredentials mirrors entree.Credentials but is received inside the
// request body for endpoints that touch multiple providers (migrate / zone
// import). Accepting credentials in the body is the pragmatic choice because
// the header scheme (D-05) only has room for one provider at a time. The slog
// middleware (D-18) never reads the body, so these values never reach logs.
type bodyCredentials struct {
	APIToken  string `json:"api_token,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	APISecret string `json:"api_secret,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
	Region    string `json:"region,omitempty"`
	Token     string `json:"token,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	AccountID string `json:"account_id,omitempty"`
}

func (b bodyCredentials) toEntreeCredentials() entree.Credentials {
	return entree.Credentials{
		APIToken:  b.APIToken,
		APIKey:    b.APIKey,
		APISecret: b.APISecret,
		AccessKey: b.AccessKey,
		SecretKey: b.SecretKey,
		Region:    b.Region,
		Token:     b.Token,
		ProjectID: b.ProjectID,
	}
}

func (b bodyCredentials) toProviderOpts() migrate.ProviderOpts {
	return migrate.ProviderOpts{
		APIToken:  b.APIToken,
		APIKey:    b.APIKey,
		APISecret: b.APISecret,
		AccessKey: b.AccessKey,
		SecretKey: b.SecretKey,
		Region:    b.Region,
		Token:     b.Token,
		ProjectID: b.ProjectID,
		AccountID: b.AccountID,
	}
}

// migrateRequest is the JSON body for POST /v1/migrate. Credentials for both
// source and target providers live here because the header scheme only has
// room for one. Values are never logged (D-18, T-06-14).
type migrateRequest struct {
	Domain             string          `json:"domain"`
	Target             string          `json:"target"`
	TargetCredentials  bodyCredentials `json:"target_credentials"`
	SourceCredentials  bodyCredentials `json:"source_credentials,omitempty"`
	PreloadedZone      *migrate.Zone   `json:"preloaded_zone,omitempty"`
	SourceNameservers  []string        `json:"source_nameservers,omitempty"`
	SourceProviderSlug string          `json:"source_provider_slug,omitempty"`
	SkipSourceDetect   bool            `json:"skip_source_detect,omitempty"`
	Labels             []string        `json:"labels,omitempty"`
	LabelsOnly         []string        `json:"labels_only,omitempty"`
	NoAXFR             bool            `json:"no_axfr,omitempty"`
	Apply              bool            `json:"apply"`
	DryRun             bool            `json:"dry_run,omitempty"`
	RatePerSecond      float64         `json:"rate_per_second,omitempty"`
	VerifyTimeoutMs    int             `json:"verify_timeout_ms,omitempty"`
	QueryTimeoutMs     int             `json:"query_timeout_ms,omitempty"`
}

// handleMigrate runs the migrate orchestrator synchronously. Long-running work
// is bounded by a context.WithTimeout derived from Server.Options.RequestTimeout
// (D-23). A deadline hit returns 500 INTERNAL "request timeout".
func (s *Server) handleMigrate(w http.ResponseWriter, r *http.Request) {
	var req migrateRequest
	if !decodeJSON(w, r, BodyLimitLarge, &req) {
		return
	}
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "domain required", nil)
		return
	}
	if req.Target == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "target required", nil)
		return
	}
	provider, err := entree.NewProvider(req.Target, req.TargetCredentials.toEntreeCredentials())
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "provider init failed", map[string]any{"target": req.Target})
		return
	}
	apply := req.Apply && !req.DryRun
	mopts := migrate.MigrateOptions{
		Domain:             req.Domain,
		TargetSlug:         req.Target,
		TargetProvider:     provider,
		ProviderOpts:       req.TargetCredentials.toProviderOpts(),
		PreloadedZone:      req.PreloadedZone,
		Apply:              apply,
		RatePerSecond:      req.RatePerSecond,
		VerifyTimeout:      time.Duration(req.VerifyTimeoutMs) * time.Millisecond,
		QueryTimeout:       time.Duration(req.QueryTimeoutMs) * time.Millisecond,
		SourceProviderSlug: req.SourceProviderSlug,
		SkipSourceDetect:   req.SkipSourceDetect,
		ScrapeOpts: migrate.ScrapeOptions{
			Nameservers: req.SourceNameservers,
			ExtraLabels: req.Labels,
			OnlyLabels:  req.LabelsOnly,
			SkipAXFR:    req.NoAXFR,
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
	defer cancel()
	done := make(chan struct{})
	var report *migrate.MigrationReport
	var runErr error
	go func() {
		defer close(done)
		report, runErr = migrate.Migrate(ctx, mopts)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeError(w, http.StatusInternalServerError, CodeInternal, "request timeout", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "request cancelled", nil)
		return
	}
	if runErr != nil {
		// Return the report alongside the error so callers can inspect
		// per-record results. Details never include credential material
		// (scrubDetails strips it defensively).
		writeError(w, http.StatusInternalServerError, CodeInternal, "migration failed", map[string]any{
			"report": report,
		})
		return
	}
	writeJSON(w, report)
}

// zoneExportRequest is the body for POST /v1/zone/export. Credentials are NOT
// required — scrape uses public DNS. Optional nameservers override.
type zoneExportRequest struct {
	Domain      string   `json:"domain"`
	Labels      []string `json:"labels,omitempty"`
	LabelsOnly  []string `json:"labels_only,omitempty"`
	NoAXFR      bool     `json:"no_axfr,omitempty"`
	Nameservers []string `json:"nameservers,omitempty"`
}

func (s *Server) handleZoneExport(w http.ResponseWriter, r *http.Request) {
	var req zoneExportRequest
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "domain required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
	defer cancel()
	zone, err := migrate.ScrapeZone(ctx, req.Domain, migrate.ScrapeOptions{
		Nameservers: req.Nameservers,
		ExtraLabels: req.Labels,
		OnlyLabels:  req.LabelsOnly,
		SkipAXFR:    req.NoAXFR,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "zone export failed", nil)
		s.logger.Warn("zone export failed", "error", err)
		return
	}
	writeJSON(w, zone)
}

// zoneImportRequest is the body for POST /v1/zone/import. 10 MiB opt-in.
type zoneImportRequest struct {
	Zone              *migrate.Zone   `json:"zone"`
	To                string          `json:"to"`
	TargetCredentials bodyCredentials `json:"target_credentials"`
	DryRun            bool            `json:"dry_run,omitempty"`
	RatePerSecond     float64         `json:"rate_per_second,omitempty"`
	VerifyTimeoutMs   int             `json:"verify_timeout_ms,omitempty"`
	QueryTimeoutMs    int             `json:"query_timeout_ms,omitempty"`
}

func (s *Server) handleZoneImport(w http.ResponseWriter, r *http.Request) {
	var req zoneImportRequest
	if !decodeJSON(w, r, BodyLimitLarge, &req) {
		return
	}
	if req.Zone == nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "zone required", nil)
		return
	}
	if req.To == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "to required", nil)
		return
	}
	if req.Zone.Domain == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "zone.domain required", nil)
		return
	}
	provider, err := entree.NewProvider(req.To, req.TargetCredentials.toEntreeCredentials())
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "provider init failed", map[string]any{"target": req.To})
		return
	}
	mopts := migrate.MigrateOptions{
		Domain:           req.Zone.Domain,
		TargetSlug:       req.To,
		TargetProvider:   provider,
		ProviderOpts:     req.TargetCredentials.toProviderOpts(),
		PreloadedZone:    req.Zone,
		Apply:            !req.DryRun,
		RatePerSecond:    req.RatePerSecond,
		VerifyTimeout:    time.Duration(req.VerifyTimeoutMs) * time.Millisecond,
		QueryTimeout:     time.Duration(req.QueryTimeoutMs) * time.Millisecond,
		SkipSourceDetect: true,
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
	defer cancel()
	done := make(chan struct{})
	var report *migrate.MigrationReport
	var runErr error
	go func() {
		defer close(done)
		report, runErr = migrate.Migrate(ctx, mopts)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeError(w, http.StatusInternalServerError, CodeInternal, "request timeout", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "request cancelled", nil)
		return
	}
	if runErr != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "zone import failed", map[string]any{"report": report})
		return
	}
	writeJSON(w, report)
}
