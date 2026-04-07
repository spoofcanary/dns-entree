package main

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"
	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/template"
)

var (
	flagCacheDir    string
	flagResolveVars []string
	flagVarsFile    string
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage Domain Connect templates (sync, list, show, resolve)",
}

var templatesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Clone or fast-forward the Domain Connect templates repo",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), flagTimeout)
		defer cancel()
		opts := templateOpts()
		if err := template.SyncTemplates(ctx, opts...); err != nil {
			return &RuntimeError{Code: "TEMPLATE_SYNC_FAILED", Msg: err.Error()}
		}
		return formatterFromCtx(cmd.Context()).EmitOK(map[string]any{
			"synced":    true,
			"cache_dir": flagCacheDir,
		})
	},
}

var templatesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cached Domain Connect templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := templateOpts()
		sums, err := template.ListTemplates(opts...)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return &UserError{
					Code: "CACHE_NOT_FOUND",
					Msg:  "template cache not found; run `entree templates sync` first",
				}
			}
			return &RuntimeError{Code: "TEMPLATE_LIST_FAILED", Msg: err.Error()}
		}
		// Strip absolute Path for determinism in goldens. Keep only ids+names.
		type listEntry struct {
			ProviderID   string `json:"provider_id"`
			ServiceID    string `json:"service_id"`
			ProviderName string `json:"provider_name,omitempty"`
			ServiceName  string `json:"service_name,omitempty"`
		}
		entries := make([]listEntry, 0, len(sums))
		for _, s := range sums {
			entries = append(entries, listEntry{
				ProviderID:   s.ProviderID,
				ServiceID:    s.ServiceID,
				ProviderName: s.ProviderName,
				ServiceName:  s.ServiceName,
			})
		}
		return formatterFromCtx(cmd.Context()).EmitOK(map[string]any{
			"templates": entries,
		})
	},
}

var templatesShowCmd = &cobra.Command{
	Use:   "show <providerID>/<serviceID>",
	Short: "Print a template as JSON",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prov, svc, err := splitTemplateSlug(args[0])
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), flagTimeout)
		defer cancel()
		t, err := template.LoadTemplate(ctx, prov, svc, templateOpts()...)
		if err != nil {
			return &UserError{
				Code: "TEMPLATE_NOT_FOUND",
				Msg:  err.Error(),
			}
		}
		return formatterFromCtx(cmd.Context()).EmitOK(templateToJSON(t))
	},
}

var templatesResolveCmd = &cobra.Command{
	Use:   "resolve <providerID>/<serviceID>",
	Short: "Resolve a template with --var/--vars-file and print the records",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prov, svc, err := splitTemplateSlug(args[0])
		if err != nil {
			return err
		}
		vars, err := ParseVarsFlags(flagResolveVars, flagVarsFile)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), flagTimeout)
		defer cancel()
		t, err := template.LoadTemplate(ctx, prov, svc, templateOpts()...)
		if err != nil {
			return &UserError{Code: "TEMPLATE_NOT_FOUND", Msg: err.Error()}
		}
		recs, err := t.Resolve(vars)
		if err != nil {
			return &UserError{Code: "VARIABLE_VALIDATION_FAILED", Msg: err.Error()}
		}
		return formatterFromCtx(cmd.Context()).EmitOK(map[string]any{
			"records": recordsToJSON(recs),
		})
	},
}

func init() {
	pf := templatesCmd.PersistentFlags()
	pf.StringVar(&flagCacheDir, "cache-dir", "", "template cache directory (default XDG cache)")

	rf := templatesResolveCmd.Flags()
	rf.StringArrayVar(&flagResolveVars, "var", nil, "template variable key=value (repeatable)")
	rf.StringVar(&flagVarsFile, "vars-file", "", "JSON file of template variables")

	templatesCmd.AddCommand(templatesSyncCmd)
	templatesCmd.AddCommand(templatesListCmd)
	templatesCmd.AddCommand(templatesShowCmd)
	templatesCmd.AddCommand(templatesResolveCmd)
	rootCmd.AddCommand(templatesCmd)
}

func templateOpts() []template.SyncOption {
	var opts []template.SyncOption
	if flagCacheDir != "" {
		opts = append(opts, template.WithCacheDir(flagCacheDir))
		// Tests seed cache dirs manually -- disable auto-sync TTL refresh.
		opts = append(opts, template.WithCacheTTL(-1))
	}
	return opts
}

func splitTemplateSlug(slug string) (string, string, error) {
	i := strings.IndexByte(slug, '/')
	if i <= 0 || i == len(slug)-1 {
		return "", "", &UserError{
			Code: "INVALID_TEMPLATE_SLUG",
			Msg:  "expected <providerID>/<serviceID>",
		}
	}
	return slug[:i], slug[i+1:], nil
}

// templateToJSON flattens a *template.Template into a deterministic map
// suitable for golden-file comparison. Private pointer fields (logger) are
// omitted automatically.
func templateToJSON(t *template.Template) map[string]any {
	recs := make([]map[string]any, 0, len(t.Records))
	for _, r := range t.Records {
		m := map[string]any{
			"type":      r.Type,
			"host":      r.Host,
			"points_to": r.PointsTo,
		}
		if r.Target != "" {
			m["target"] = r.Target
		}
		if r.Data != "" {
			m["data"] = r.Data
		}
		if r.GroupID != "" {
			m["group_id"] = r.GroupID
		}
		if r.Essential != "" {
			m["essential"] = r.Essential
		}
		if r.TxtConflictMatchingMode != "" {
			m["txt_conflict_matching_mode"] = r.TxtConflictMatchingMode
		}
		if r.TxtConflictMatchingPrefix != "" {
			m["txt_conflict_matching_prefix"] = r.TxtConflictMatchingPrefix
		}
		recs = append(recs, m)
	}
	return map[string]any{
		"provider_id":   t.ProviderID,
		"provider_name": t.ProviderName,
		"service_id":    t.ServiceID,
		"service_name":  t.ServiceName,
		"version":       t.Version,
		"description":   t.Description,
		"records":       recs,
	}
}

func recordsToJSON(recs []entree.Record) []map[string]any {
	out := make([]map[string]any, 0, len(recs))
	for _, r := range recs {
		m := map[string]any{
			"type":    r.Type,
			"name":    r.Name,
			"content": r.Content,
			"ttl":     r.TTL,
		}
		if r.Priority != 0 {
			m["priority"] = r.Priority
		}
		out = append(out, m)
	}
	return out
}
