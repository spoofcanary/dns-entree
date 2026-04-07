package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/template"
)

// applyTemplateRun is the RunE implementation for `entree apply <domain>
// --template <providerID>/<serviceID> ...`. 05-02 owns the apply command and
// calls this hook when --template is set. Exposed via runTemplateBranch so
// 05-02 and 05-03 can be developed in parallel.
func applyTemplateRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return &UserError{Code: "INVALID_DOMAIN", Msg: "apply requires a domain argument"}
	}
	domain := args[0]

	tmplSlug, _ := cmd.Flags().GetString("template")
	varFlags, _ := cmd.Flags().GetStringArray("var")
	varsFile, _ := cmd.Flags().GetString("vars-file")
	cacheDir, _ := cmd.Flags().GetString("cache-dir")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Non-TTY write path requires --yes (D-11).
	if !dryRun {
		if err := RequireYes(IsTTY(os.Stdout), flagYes); err != nil {
			return err
		}
	}

	prov, svc, err := splitTemplateSlug(tmplSlug)
	if err != nil {
		return err
	}

	var opts []template.SyncOption
	if cacheDir != "" {
		opts = append(opts, template.WithCacheDir(cacheDir), template.WithCacheTTL(-1))
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), flagTimeout)
	defer cancel()

	t, err := template.LoadTemplate(ctx, prov, svc, opts...)
	if err != nil {
		return &UserError{Code: "TEMPLATE_NOT_FOUND", Msg: err.Error()}
	}

	vars, err := ParseVarsFlags(varFlags, varsFile)
	if err != nil {
		return err
	}

	recs, err := t.Resolve(vars)
	if err != nil {
		return &UserError{Code: "VARIABLE_VALIDATION_FAILED", Msg: err.Error()}
	}

	f := formatterFromCtx(cmd.Context())

	if dryRun {
		return f.EmitOK(map[string]any{
			"domain":  domain,
			"records": recordsToJSON(recs),
		})
	}

	// Write path: 05-02 is responsible for constructing the provider +
	// PushService and invoking template.ApplyTemplate. Since 05-02 owns that
	// wiring, we delegate here via a second hook. If the hook is unset (plan
	// 05-02 not yet merged), we return a runtime error so users know to update.
	if applyTemplateExecutor == nil {
		applyTemplateExecutor = defaultApplyTemplateExecutor
	}
	results, err := applyTemplateExecutor(ctx, domain, t, vars)
	if err != nil {
		return &RuntimeError{Code: "APPLY_FAILED", Msg: err.Error(), Details: map[string]any{
			"results": pushResultsToJSON(results),
		}}
	}
	return f.EmitOK(map[string]any{
		"domain":  domain,
		"results": pushResultsToJSON(results),
	})
}

// applyTemplateExecutor is the write-path seam. Defaults to
// defaultApplyTemplateExecutor which builds a real provider + PushService from
// the apply command's --provider / --credentials-file flags. Tests may
// override it.
var applyTemplateExecutor func(ctx context.Context, domain string, t *template.Template, vars map[string]string) ([]*entree.PushResult, error)

func defaultApplyTemplateExecutor(ctx context.Context, domain string, t *template.Template, vars map[string]string) ([]*entree.PushResult, error) {
	slug := flagProvider
	if slug == "" {
		det, err := entree.DetectProvider(ctx, domain)
		if err != nil {
			return nil, fmt.Errorf("detect provider: %w", err)
		}
		if !det.Supported {
			return nil, fmt.Errorf("provider %q not supported; pass --provider", det.Provider)
		}
		slug = string(det.Provider)
	}
	creds, err := NewCredentialsLoader(flagCredentialsFile).Load(slug)
	if err != nil {
		return nil, err
	}
	prov, err := entree.NewProvider(slug, creds)
	if err != nil {
		return nil, fmt.Errorf("provider init: %w", err)
	}
	push := entree.NewPushService(prov)
	return template.ApplyTemplate(ctx, push, domain, t, vars)
}

func pushResultsToJSON(results []*entree.PushResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		if r == nil {
			continue
		}
		m := map[string]any{
			"status": string(r.Status),
			"name":   r.RecordName,
			"value":  r.RecordValue,
		}
		if r.PreviousValue != "" {
			m["previous"] = r.PreviousValue
		}
		if r.VerifyError != nil {
			m["verify_error"] = r.VerifyError.Error()
		} else {
			m["verified"] = r.Verified
		}
		out = append(out, m)
	}
	return out
}

func init() {
	runTemplateBranch = applyTemplateRun
}
