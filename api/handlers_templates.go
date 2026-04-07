package api

import (
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/spoofcanary/dns-entree/template"
)

// handleTemplatesSync runs template.SyncTemplates against the server's
// configured cache dir. Network heavy — callers should expect seconds.
func (s *Server) handleTemplatesSync(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	opts := s.templateOpts()
	if err := template.SyncTemplates(r.Context(), opts...); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "template sync failed", nil)
		s.logger.Error("template sync failed", "error", err)
		return
	}
	summaries, err := template.ListTemplates(opts...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "template list failed", nil)
		s.logger.Error("template list failed", "error", err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "count": len(summaries)})
}

// handleTemplatesList returns the flat list of cached templates.
func (s *Server) handleTemplatesList(w http.ResponseWriter, r *http.Request) {
	opts := s.templateOpts()
	// Use a negative TTL so List does not force a network sync.
	summaries, err := template.ListTemplates(append(opts, template.WithCacheTTL(-1))...)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeJSON(w, []template.TemplateSummary{})
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "template list failed", nil)
		s.logger.Error("template list failed", "error", err)
		return
	}
	writeJSON(w, summaries)
}

// handleTemplateGet loads one template by provider/service.
func (s *Server) handleTemplateGet(w http.ResponseWriter, r *http.Request) {
	provider := strings.TrimSpace(r.PathValue("provider"))
	service := strings.TrimSpace(r.PathValue("service"))
	if provider == "" || service == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "provider and service required", nil)
		return
	}
	opts := append(s.templateOpts(), template.WithCacheTTL(-1))
	tpl, err := template.LoadTemplate(r.Context(), provider, service, opts...)
	if err != nil {
		writeError(w, http.StatusNotFound, CodeTemplateNotFound, "template not found", map[string]any{
			"provider": provider,
			"service":  service,
		})
		return
	}
	writeJSON(w, tpl)
}

// handleTemplateResolve resolves a template's variables into concrete records.
func (s *Server) handleTemplateResolve(w http.ResponseWriter, r *http.Request) {
	provider := strings.TrimSpace(r.PathValue("provider"))
	service := strings.TrimSpace(r.PathValue("service"))
	if provider == "" || service == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "provider and service required", nil)
		return
	}
	var req struct {
		Vars map[string]string `json:"vars"`
	}
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	opts := append(s.templateOpts(), template.WithCacheTTL(-1))
	tpl, err := template.LoadTemplate(r.Context(), provider, service, opts...)
	if err != nil {
		writeError(w, http.StatusNotFound, CodeTemplateNotFound, "template not found", map[string]any{
			"provider": provider,
			"service":  service,
		})
		return
	}
	resolved, err := tpl.ResolveDetailed(req.Vars)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	writeJSON(w, resolved)
}

func (s *Server) templateOpts() []template.SyncOption {
	var opts []template.SyncOption
	if s.opts.TemplateCacheDir != "" {
		opts = append(opts, template.WithCacheDir(s.opts.TemplateCacheDir))
	}
	return opts
}
