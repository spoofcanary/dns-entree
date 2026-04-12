package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/migrate"
)

// Stateful migration handlers (Phase 07-04). Implements the
// preview -> apply -> verify workflow backed by migrate.MigrationStore.
// Routes and production wiring (Options fields, sweeper) are added in 07-05.
//
// Per-migration access_token is issued at preview and compared via
// crypto/subtle.ConstantTimeCompare on every subsequent call (D-14, T-07-10).

// Code constants local to this file so we do not bump the public error-code
// set until 07-05. Wave 07-05 promotes these to api/errors.go if needed.
const (
	codeConflict     = "CONFLICT"
	codeUnauthorized = "UNAUTHORIZED"
	codeNotFound     = "NOT_FOUND"
	codeGone         = "GONE"
)

// migratePreviewRequest is the body for POST /v1/migrate/preview.
// PreloadedZone is an optional test seam: when set, ScrapeZone is skipped.
// Production callers omit it and let the handler scrape.
type migratePreviewRequest struct {
	Domain            string          `json:"domain"`
	Target            string          `json:"target"`
	TenantID          string          `json:"tenant_id,omitempty"`
	TargetCredentials bodyCredentials `json:"target_credentials"`
	SourceNameservers []string        `json:"source_nameservers,omitempty"`
	Labels            []string        `json:"labels,omitempty"`
	LabelsOnly        []string        `json:"labels_only,omitempty"`
	NoAXFR            bool            `json:"no_axfr,omitempty"`
	PreloadedZone     *migrate.Zone   `json:"preloaded_zone,omitempty"`
}

// issueAccessToken returns 32 random bytes base64 URL-encoded (no padding).
// ~256 bits of entropy.
func issueAccessToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func checkAccessToken(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// redactCreds returns a shallow copy of m with CredentialBlob and AccessToken
// cleared (T-07-12, T-07-13).
func redactCreds(m *migrate.StoredMigration) *migrate.StoredMigration {
	if m == nil {
		return nil
	}
	c := *m
	c.CredentialBlob = nil
	c.AccessToken = ""
	return &c
}

// extractPathID extracts the migration id from a path of the form
// /v1/migrate/{id} or /v1/migrate/{id}/action.
func extractPathID(path string) string {
	// trim prefix
	p := strings.TrimPrefix(path, "/v1/migrate/")
	if p == path {
		return ""
	}
	// take segment before the next slash
	if i := strings.Index(p, "/"); i >= 0 {
		return p[:i]
	}
	return p
}

// loadAndAuth loads a migration row, checks it exists, has not expired, and
// that the Bearer token matches. On failure it writes the response and
// returns (nil, false).
func (s *Server) loadAndAuth(w http.ResponseWriter, r *http.Request, id string) (*migrate.StoredMigration, bool) {
	if id == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "id required", nil)
		return nil, false
	}
	store := s.migrationStore
	if store == nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "migration store not configured", nil)
		return nil, false
	}
	row, err := store.Get(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, migrate.ErrNotFound):
			writeError(w, http.StatusNotFound, codeNotFound, "migration not found", nil)
		case errors.Is(err, migrate.ErrExpired):
			writeError(w, http.StatusGone, codeGone, "migration expired", nil)
		default:
			s.logger.Warn("migration load failed", "error", err, "id", id)
			writeError(w, http.StatusInternalServerError, CodeInternal, "load failed", nil)
		}
		return nil, false
	}
	// Double-check expiry even if backend didn't flag it.
	if !row.ExpiresAt.IsZero() && row.ExpiresAt.Before(time.Now().UTC()) {
		writeError(w, http.StatusGone, codeGone, "migration expired", nil)
		return nil, false
	}
	tok := extractBearer(r)
	if tok == "" {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "missing bearer token", nil)
		return nil, false
	}
	if !checkAccessToken(tok, row.AccessToken) {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "invalid bearer token", nil)
		return nil, false
	}
	return row, true
}

// handleMigratePreview implements POST /v1/migrate/preview.
// Scrapes the source zone (or uses PreloadedZone in tests), encrypts the
// target credentials with the server state key, issues an access token, and
// persists a StoredMigration row with Status=preview. Does NOT call
// EnsureZone - apply is the first target-side write (D-12, T-07-11).
func (s *Server) handleMigratePreview(w http.ResponseWriter, r *http.Request) {
	var req migratePreviewRequest
	if !decodeJSON(w, r, BodyLimitLarge, &req) {
		return
	}
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "domain required", nil)
		return
	}
	if err := entree.ValidateDNSName(req.Domain); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid domain: "+err.Error(), nil)
		return
	}
	if req.Target == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "target required", nil)
		return
	}
	if _, err := migrate.GetAdapter(req.Target); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "unknown target provider", map[string]any{"target": req.Target})
		return
	}
	if s.migrationStore == nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "migration store not configured", nil)
		return
	}
	if len(s.migrationKey) != 32 {
		writeError(w, http.StatusInternalServerError, CodeInternal, "state key not configured", nil)
		return
	}

	timeout := s.opts.RequestTimeout
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	var zone *migrate.Zone
	if req.PreloadedZone != nil {
		zone = req.PreloadedZone
	} else {
		z, err := migrate.ScrapeZone(ctx, req.Domain, migrate.ScrapeOptions{
			Nameservers: req.SourceNameservers,
			ExtraLabels: req.Labels,
			OnlyLabels:  req.LabelsOnly,
			SkipAXFR:    req.NoAXFR,
		})
		if err != nil {
			s.logger.Warn("preview scrape failed", "error", err, "domain", req.Domain)
			writeError(w, http.StatusInternalServerError, CodeInternal, "scrape failed", nil)
			return
		}
		zone = z
	}

	// Best-effort source detection.
	var srcSlug string
	if det, err := entree.DetectProvider(ctx, req.Domain); err == nil && det != nil {
		srcSlug = string(det.Provider)
	}

	credJSON, err := json.Marshal(req.TargetCredentials)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "encode credentials", nil)
		return
	}
	blob, err := migrate.EncryptCreds(s.migrationKey, credJSON)
	if err != nil {
		s.logger.Error("encrypt creds failed", "error", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, "encrypt credentials", nil)
		return
	}

	ttl := s.migrationTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	now := time.Now().UTC()
	row := &migrate.StoredMigration{
		ID:             migrate.NewMigrationID(),
		TenantID:       req.TenantID,
		Status:         migrate.StatusPreview,
		Domain:         req.Domain,
		SourceSlug:     srcSlug,
		TargetSlug:     req.Target,
		Preview:        zone,
		PreviewRecords: zone.Records,
		CredentialBlob: blob,
		AccessToken:    issueAccessToken(),
		ExpiresAt:      now.Add(ttl),
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.migrationStore.Create(ctx, row); err != nil {
		s.logger.Error("store create failed", "error", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, "persist migration", nil)
		return
	}

	writeJSON(w, map[string]any{
		"id":              row.ID,
		"access_token":    row.AccessToken,
		"status":          row.Status,
		"domain":          row.Domain,
		"target":          row.TargetSlug,
		"detected_source": srcSlug,
		"preview":         zone,
		"expires_at":      row.ExpiresAt,
	})
}

// handleMigrateApply implements POST /v1/migrate/{id}/apply.
// Transitions preview -> applying (Version+1) BEFORE any provider work so a
// concurrent second apply hits ErrVersionMismatch and returns 409 (T-07-11).
func (s *Server) handleMigrateApply(w http.ResponseWriter, r *http.Request) {
	id := extractPathID(r.URL.Path)
	row, ok := s.loadAndAuth(w, r, id)
	if !ok {
		return
	}
	if row.Status != migrate.StatusPreview {
		writeError(w, http.StatusConflict, codeConflict, "migration not in preview state", map[string]any{"status": row.Status})
		return
	}

	timeout := s.opts.RequestTimeout
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Decrypt credentials.
	credJSON, err := migrate.DecryptCreds(s.migrationKey, row.CredentialBlob)
	if err != nil {
		s.logger.Error("decrypt creds failed", "error", err, "id", row.ID)
		writeError(w, http.StatusInternalServerError, CodeInternal, "decrypt credentials", nil)
		return
	}
	var creds bodyCredentials
	if err := json.Unmarshal(credJSON, &creds); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "credentials corrupt", nil)
		return
	}

	adapter, err := migrate.GetAdapter(row.TargetSlug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "adapter lookup failed", nil)
		return
	}

	// STATE TRANSITION: preview -> applying, Version++ BEFORE provider work.
	row.Status = migrate.StatusApplying
	row.Version++
	if err := s.migrationStore.Update(ctx, row); err != nil {
		if errors.Is(err, migrate.ErrVersionMismatch) {
			writeError(w, http.StatusConflict, codeConflict, "concurrent modification", nil)
			return
		}
		s.logger.Error("store update(applying) failed", "error", err, "id", row.ID)
		writeError(w, http.StatusInternalServerError, CodeInternal, "persist state", nil)
		return
	}

	// Provider work.
	zi, err := adapter.EnsureZone(ctx, row.Domain, creds.toProviderOpts())
	if err != nil {
		row.Status = migrate.StatusFailed
		row.ErrorMessage = err.Error()
		row.Version++
		_ = s.migrationStore.Update(ctx, row)
		s.logger.Warn("EnsureZone failed", "error", err, "id", row.ID)
		writeError(w, http.StatusInternalServerError, CodeInternal, "ensure zone failed", nil)
		return
	}
	row.TargetZoneID = zi.ZoneID
	row.TargetNameservers = zi.Nameservers
	row.NSChangeInstructions = migrate.FormatNSChangeInstructions(row.SourceSlug, zi.Nameservers)

	provider, err := entree.NewProvider(row.TargetSlug, creds.toEntreeCredentials())
	if err != nil {
		row.Status = migrate.StatusFailed
		row.ErrorMessage = err.Error()
		row.Version++
		_ = s.migrationStore.Update(ctx, row)
		writeError(w, http.StatusInternalServerError, CodeInternal, "provider init failed", nil)
		return
	}

	limiter := migrate.NewWriteLimiter(s.migrationRatePerSecond)
	results := make([]*entree.PushResult, 0, len(row.PreviewRecords))
	var firstErr string
	for i := range row.PreviewRecords {
		rec := row.PreviewRecords[i]
		if err := limiter.Wait(ctx); err != nil {
			firstErr = err.Error()
			break
		}
		res := &entree.PushResult{RecordName: rec.Name, RecordValue: rec.Content}
		if err := provider.SetRecord(ctx, row.Domain, rec); err != nil {
			res.Status = entree.StatusFailed
			res.VerifyError = err
			if firstErr == "" {
				firstErr = err.Error()
			}
		} else {
			res.Status = entree.StatusUpdated
		}
		results = append(results, res)
	}
	row.ApplyResults = results
	if firstErr != "" {
		row.Status = migrate.StatusFailed
		row.ErrorMessage = firstErr
	} else {
		row.Status = migrate.StatusAwaitingNSChange
	}
	row.Version++
	if err := s.migrationStore.Update(ctx, row); err != nil {
		if errors.Is(err, migrate.ErrVersionMismatch) {
			writeError(w, http.StatusConflict, codeConflict, "concurrent modification", nil)
			return
		}
		s.logger.Error("store update(applied) failed", "error", err, "id", row.ID)
		writeError(w, http.StatusInternalServerError, CodeInternal, "persist state", nil)
		return
	}

	writeJSON(w, map[string]any{
		"status":               row.Status,
		"target_nameservers":   row.TargetNameservers,
		"ns_instructions":      row.NSChangeInstructions,
		"apply_results":        row.ApplyResults,
		"applied_record_count": len(row.ApplyResults),
	})
}

// handleMigrateVerify implements POST /v1/migrate/{id}/verify.
// Runs a single VerifyAgainstNS round. Transitions to complete when all
// records match.
func (s *Server) handleMigrateVerify(w http.ResponseWriter, r *http.Request) {
	id := extractPathID(r.URL.Path)
	row, ok := s.loadAndAuth(w, r, id)
	if !ok {
		return
	}
	switch row.Status {
	case migrate.StatusApplying, migrate.StatusAwaitingNSChange, migrate.StatusVerifying, migrate.StatusComplete:
	default:
		writeError(w, http.StatusConflict, codeConflict, "migration not in verifiable state", map[string]any{"status": row.Status})
		return
	}

	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
	defer cancel()

	summary := migrate.VerifyAgainstNS(ctx, row.TargetNameservers, row.PreviewRecords, timeout)
	if summary.Total > 0 && summary.Matched == summary.Total {
		row.Status = migrate.StatusComplete
	} else {
		row.Status = migrate.StatusVerifying
	}
	row.Version++
	if err := s.migrationStore.Update(ctx, row); err != nil {
		if errors.Is(err, migrate.ErrVersionMismatch) {
			writeError(w, http.StatusConflict, codeConflict, "concurrent modification", nil)
			return
		}
		s.logger.Error("store update(verify) failed", "error", err, "id", row.ID)
		writeError(w, http.StatusInternalServerError, CodeInternal, "persist state", nil)
		return
	}
	writeJSON(w, map[string]any{
		"status":  row.Status,
		"matched": summary.Matched,
		"total":   summary.Total,
		"details": summary.Results,
	})
}

// handleMigrateGet implements GET /v1/migrate/{id}. Bearer-auth required.
// CredentialBlob and AccessToken are redacted on the returned row.
func (s *Server) handleMigrateGet(w http.ResponseWriter, r *http.Request) {
	id := extractPathID(r.URL.Path)
	row, ok := s.loadAndAuth(w, r, id)
	if !ok {
		return
	}
	writeJSON(w, redactCreds(row))
}

// handleMigrateList implements GET /v1/migrate. NO auth per orchestrator
// decision (D-19): operators must front this endpoint with a reverse proxy.
// Every row has credentials and access token redacted before serialization.
func (s *Server) handleMigrateList(w http.ResponseWriter, r *http.Request) {
	if s.migrationStore == nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "migration store not configured", nil)
		return
	}
	filter := migrate.ListFilter{
		TenantID: r.URL.Query().Get("tenant_id"),
		Status:   r.URL.Query().Get("status"),
	}
	rows, err := s.migrationStore.List(r.Context(), filter)
	if err != nil {
		s.logger.Warn("list failed", "error", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, "list failed", nil)
		return
	}
	out := make([]*migrate.StoredMigration, 0, len(rows))
	for _, row := range rows {
		out = append(out, redactCreds(row))
	}
	writeJSON(w, out)
}

// handleMigrateDelete implements DELETE /v1/migrate/{id}. Bearer-auth required.
func (s *Server) handleMigrateDelete(w http.ResponseWriter, r *http.Request) {
	id := extractPathID(r.URL.Path)
	row, ok := s.loadAndAuth(w, r, id)
	if !ok {
		return
	}
	if err := s.migrationStore.Delete(r.Context(), row.ID); err != nil {
		s.logger.Warn("delete failed", "error", err, "id", row.ID)
		writeError(w, http.StatusInternalServerError, CodeInternal, "delete failed", nil)
		return
	}
	writeJSON(w, map[string]any{"deleted": true})
}
