package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/template"
)

// Test seams so unit tests can stub network I/O. Production defaults call
// the real library.
var (
	detectProviderFn = entree.DetectProvider
	verifyFn         = entree.Verify
	loadTemplateFn   = template.LoadTemplate
	applyTemplateFn  = template.ApplyTemplate
)

// ----- request/response shapes -----------------------------------------------

type detectRequest struct {
	Domain string `json:"domain"`
}

type detectResponse struct {
	Provider    string   `json:"provider"`
	Label       string   `json:"label"`
	Supported   bool     `json:"supported"`
	Nameservers []string `json:"nameservers"`
	Method      string   `json:"method"`
}

type verifyRequest struct {
	Domain   string `json:"domain"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Contains string `json:"contains"`
}

type verifyResponse struct {
	Verified           bool     `json:"verified"`
	CurrentValue       string   `json:"current_value"`
	Method             string   `json:"method"`
	NameserversQueried []string `json:"nameservers_queried"`
}

type spfMergeRequest struct {
	Current  string   `json:"current"`
	Includes []string `json:"includes"`
}

type spfMergeResponse struct {
	Value               string   `json:"value"`
	Changed             bool     `json:"changed"`
	BrokenInput         bool     `json:"broken_input"`
	LookupCount         int      `json:"lookup_count"`
	LookupLimitExceeded bool     `json:"lookup_limit_exceeded"`
	Warnings            []string `json:"warnings,omitempty"`
}

type applyRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

type applyRequest struct {
	Domain  string        `json:"domain"`
	Records []applyRecord `json:"records"`
	DryRun  bool          `json:"dry_run"`
}

type applyDiffEntry struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Current  string `json:"current"`
	Proposed string `json:"proposed"`
	Action   string `json:"action"`
}

type applyResultEntry struct {
	Type          string `json:"type"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	RecordValue   string `json:"record_value"`
	PreviousValue string `json:"previous_value,omitempty"`
	Verified      bool   `json:"verified"`
	VerifyError   string `json:"verify_error,omitempty"`
}

type applyReport struct {
	Domain  string             `json:"domain"`
	DryRun  bool               `json:"dry_run"`
	Diffs   []applyDiffEntry   `json:"diffs,omitempty"`
	Results []applyResultEntry `json:"results,omitempty"`
}

type applyTemplateRequest struct {
	Domain     string            `json:"domain"`
	ProviderID string            `json:"provider_id"`
	ServiceID  string            `json:"service_id"`
	Vars       map[string]string `json:"vars"`
	DryRun     bool              `json:"dry_run"`
}

// ----- helpers ----------------------------------------------------------------

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeError(w, http.StatusMethodNotAllowed, CodeBadRequest, "method not allowed", nil)
		return false
	}
	return true
}

func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		writeError(w, http.StatusUnsupportedMediaType, CodeBadRequest, "content-type required", nil)
		return false
	}
	// Allow "application/json" or "application/json; charset=..."
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if !strings.EqualFold(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, CodeBadRequest, "content-type must be application/json", nil)
		return false
	}
	return true
}

// decodeJSON wraps r.Body in MaxBytesReader and decodes into v. Returns true on
// success; on failure it writes the appropriate envelope and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, CodeBadRequest, "request body too large", nil)
			return false
		}
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid JSON body", nil)
		return false
	}
	return true
}

func reqID(r *http.Request) string {
	v, _ := r.Context().Value(ctxKeyRequestID).(string)
	return v
}

// ----- handlers ---------------------------------------------------------------

func (s *Server) handleDetect(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireJSON(w, r) {
		return
	}
	var req detectRequest
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "domain is required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
	defer cancel()
	res, err := detectProviderFn(ctx, req.Domain)
	if err != nil {
		s.logger.Error("detect failed", "error", err.Error(), "request_id", reqID(r), "domain", req.Domain)
		writeError(w, http.StatusInternalServerError, CodeProviderError, "detection failed", nil)
		return
	}
	writeJSON(w, detectResponse{
		Provider:    string(res.Provider),
		Label:       res.Label,
		Supported:   res.Supported,
		Nameservers: res.Nameservers,
		Method:      res.Method,
	})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireJSON(w, r) {
		return
	}
	var req verifyRequest
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	if req.Domain == "" || req.Type == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "domain, type, and name are required", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
	defer cancel()
	res, err := verifyFn(ctx, req.Domain, entree.VerifyOpts{
		RecordType: req.Type,
		Name:       req.Name,
		Contains:   req.Contains,
	})
	if err != nil {
		s.logger.Error("verify failed", "error", err.Error(), "request_id", reqID(r), "domain", req.Domain)
		writeError(w, http.StatusInternalServerError, CodeProviderError, "verification failed", nil)
		return
	}
	payload := verifyResponse{
		Verified:           res.Verified,
		CurrentValue:       res.CurrentValue,
		Method:             res.Method,
		NameserversQueried: res.NameserversQueried,
	}
	if !res.Verified {
		writeError(w, http.StatusConflict, CodeVerifyMismatch, "verification did not match", map[string]any{
			"verified":            false,
			"current_value":       payload.CurrentValue,
			"method":              payload.Method,
			"nameservers_queried": payload.NameserversQueried,
		})
		return
	}
	writeJSON(w, payload)
}

func (s *Server) handleSPFMerge(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireJSON(w, r) {
		return
	}
	var req spfMergeRequest
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	res, err := entree.MergeSPF(req.Current, req.Includes)
	if err != nil {
		s.logger.Error("spf merge failed", "error", err.Error(), "request_id", reqID(r))
		writeError(w, http.StatusBadRequest, CodeBadRequest, "spf merge failed", nil)
		return
	}
	writeJSON(w, spfMergeResponse{
		Value:               res.Value,
		Changed:             res.Changed,
		BrokenInput:         res.BrokenInput,
		LookupCount:         res.LookupCount,
		LookupLimitExceeded: res.LookupLimitExceeded,
		Warnings:            res.Warnings,
	})
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireJSON(w, r) {
		return
	}
	var req applyRequest
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	if req.Domain == "" || len(req.Records) == 0 {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "domain and at least one record are required", nil)
		return
	}
	slug, creds, credErr := parseCredentialHeaders(r)
	if credErr != nil {
		credErr.writeTo(w)
		return
	}
	provider, err := entree.NewProvider(slug, creds)
	if err != nil {
		s.logger.Error("provider init failed", "error", err.Error(), "request_id", reqID(r), "provider", slug)
		writeError(w, http.StatusInternalServerError, CodeProviderError, "provider init failed", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
	defer cancel()

	report := applyReport{Domain: req.Domain, DryRun: req.DryRun}

	if req.DryRun {
		for _, rec := range req.Records {
			existing, gerr := provider.GetRecords(ctx, req.Domain, rec.Type)
			if gerr != nil {
				s.logger.Error("get records failed", "error", gerr.Error(), "request_id", reqID(r), "domain", req.Domain)
				writeError(w, http.StatusInternalServerError, CodeProviderError, "apply failed", nil)
				return
			}
			entry := applyDiffEntry{Type: rec.Type, Name: rec.Name, Proposed: rec.Content, Action: "CREATE"}
			for _, e := range existing {
				if e.Name == rec.Name {
					entry.Current = e.Content
					if e.Content == rec.Content {
						entry.Action = "SKIP"
					} else {
						entry.Action = "UPDATE"
					}
					break
				}
			}
			report.Diffs = append(report.Diffs, entry)
		}
		writeJSON(w, report)
		return
	}

	push := entree.NewPushService(provider)
	anyFailed := false
	for _, rec := range req.Records {
		entry := applyResultEntry{Type: rec.Type, Name: rec.Name, RecordValue: rec.Content}
		var res *entree.PushResult
		var perr error
		switch rec.Type {
		case "TXT":
			res, perr = push.PushTXTRecord(ctx, req.Domain, rec.Name, rec.Content)
		case "CNAME":
			res, perr = push.PushCNAMERecord(ctx, req.Domain, rec.Name, rec.Content)
		case "A", "AAAA", "MX", "NS", "SRV":
			r2 := entree.Record{Type: rec.Type, Name: rec.Name, Content: rec.Content, TTL: rec.TTL}
			res, perr = push.PushGenericRecord(ctx, req.Domain, r2)
		default:
			perr = errors.New("unsupported record type")
		}
		if perr != nil {
			s.logger.Error("apply record failed",
				"error", perr.Error(),
				"request_id", reqID(r),
				"domain", req.Domain,
				"record_name", rec.Name,
				"record_type", rec.Type,
			)
			entry.Status = string(entree.StatusFailed)
			anyFailed = true
		} else {
			entry.Status = string(res.Status)
			entry.PreviousValue = res.PreviousValue
			entry.Verified = res.Verified
			if res.VerifyError != nil {
				entry.VerifyError = "post-push verification did not observe record"
			}
		}
		report.Results = append(report.Results, entry)
	}

	if anyFailed {
		writeError(w, http.StatusInternalServerError, CodeProviderError, "apply failed", map[string]any{
			"results": report.Results,
		})
		return
	}
	writeJSON(w, report)
}

func (s *Server) handleApplyTemplate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !requireJSON(w, r) {
		return
	}
	var req applyTemplateRequest
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	if req.Domain == "" || req.ProviderID == "" || req.ServiceID == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "domain, provider_id, and service_id are required", nil)
		return
	}
	slug, creds, credErr := parseCredentialHeaders(r)
	if credErr != nil {
		credErr.writeTo(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
	defer cancel()

	var opts []template.SyncOption
	if s.opts.TemplateCacheDir != "" {
		opts = append(opts, template.WithCacheDir(s.opts.TemplateCacheDir))
	}
	tmpl, err := loadTemplateFn(ctx, req.ProviderID, req.ServiceID, opts...)
	if err != nil {
		s.logger.Error("template load failed", "error", err.Error(), "request_id", reqID(r), "provider_id", req.ProviderID, "service_id", req.ServiceID)
		writeError(w, http.StatusNotFound, CodeTemplateNotFound, "template not found", nil)
		return
	}

	if req.DryRun {
		recs, rerr := tmpl.Resolve(req.Vars)
		if rerr != nil {
			s.logger.Error("template resolve failed", "error", rerr.Error(), "request_id", reqID(r))
			writeError(w, http.StatusBadRequest, CodeBadRequest, "template resolve failed", nil)
			return
		}
		out := make([]map[string]any, 0, len(recs))
		for _, rec := range recs {
			out = append(out, map[string]any{
				"type":    rec.Type,
				"name":    rec.Name,
				"content": rec.Content,
				"ttl":     rec.TTL,
			})
		}
		writeJSON(w, map[string]any{
			"domain":  req.Domain,
			"dry_run": true,
			"records": out,
		})
		return
	}

	provider, err := entree.NewProvider(slug, creds)
	if err != nil {
		s.logger.Error("provider init failed", "error", err.Error(), "request_id", reqID(r), "provider", slug)
		writeError(w, http.StatusInternalServerError, CodeProviderError, "provider init failed", nil)
		return
	}
	push := entree.NewPushService(provider)
	results, aerr := applyTemplateFn(ctx, push, req.Domain, tmpl, req.Vars)
	if aerr != nil {
		s.logger.Error("template apply failed", "error", aerr.Error(), "request_id", reqID(r), "domain", req.Domain)
		writeError(w, http.StatusInternalServerError, CodeProviderError, "template apply failed", map[string]any{
			"results": pushResultsToJSON(results),
		})
		return
	}
	writeJSON(w, map[string]any{
		"domain":  req.Domain,
		"dry_run": false,
		"results": pushResultsToJSON(results),
	})
}

func pushResultsToJSON(results []*entree.PushResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		if r == nil {
			continue
		}
		m := map[string]any{
			"status":   string(r.Status),
			"name":     r.RecordName,
			"value":    r.RecordValue,
			"verified": r.Verified,
		}
		if r.PreviousValue != "" {
			m["previous"] = r.PreviousValue
		}
		if r.VerifyError != nil {
			m["verify_error"] = "post-push verification did not observe record"
		}
		out = append(out, m)
	}
	return out
}

// keep io imported for MaxBytesReader use in decodeJSON.
var _ io.Reader = (*strings.Reader)(nil)
