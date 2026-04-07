package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spf13/cobra"
)

// runTemplateBranch is declared in coordination_hook.go and populated by 05-03.

var (
	flagApplyRecords  []string
	flagApplyDryRun   bool
	flagApplyTemplate string
	flagApplyVars     []string
	flagApplyVarsFile string
)

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
	Error         string `json:"error,omitempty"`
}

type applyDiffData struct {
	Records []applyDiffEntry `json:"records"`
}

type applyResultData struct {
	Results []applyResultEntry `json:"results"`
}

type parsedRecord struct {
	Type    string
	Name    string
	Content string
}

func parseRecordSpec(spec string) (parsedRecord, error) {
	// TYPE:NAME:VALUE -- VALUE may contain ':'. Split on first two colons only.
	i := strings.Index(spec, ":")
	if i < 0 {
		return parsedRecord{}, fmt.Errorf("expected TYPE:NAME:VALUE, got %q", spec)
	}
	rest := spec[i+1:]
	j := strings.Index(rest, ":")
	if j < 0 {
		return parsedRecord{}, fmt.Errorf("expected TYPE:NAME:VALUE, got %q", spec)
	}
	pr := parsedRecord{
		Type:    spec[:i],
		Name:    rest[:j],
		Content: rest[j+1:],
	}
	if pr.Type == "" || pr.Name == "" || pr.Content == "" {
		return parsedRecord{}, fmt.Errorf("empty field in %q", spec)
	}
	if !isApplyRecordType(pr.Type) {
		return parsedRecord{}, fmt.Errorf("unsupported type %q", pr.Type)
	}
	if strings.ContainsAny(pr.Name, "\n\x00") || strings.ContainsAny(pr.Content, "\n\x00") {
		return parsedRecord{}, fmt.Errorf("control character in record spec")
	}
	return pr, nil
}

func isApplyRecordType(t string) bool {
	switch t {
	case "TXT", "CNAME", "MX", "A", "AAAA", "SRV":
		return true
	}
	return false
}

var applyCmd = &cobra.Command{
	Use:   "apply <domain>",
	Short: "Apply records or a Domain Connect template to a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagApplyTemplate != "" {
			if runTemplateBranch == nil {
				return &UserError{Code: "TEMPLATE_UNSUPPORTED", Msg: "template support not compiled in"}
			}
			return runTemplateBranch(cmd, args)
		}
		return runRecordBranch(cmd, args)
	},
}

func runRecordBranch(cmd *cobra.Command, args []string) error {
	domain := args[0]
	if domain == "" {
		return &UserError{Code: "INVALID_DOMAIN", Msg: "domain required"}
	}
	if len(flagApplyRecords) == 0 {
		return &UserError{Code: "NO_RECORDS", Msg: "at least one --record required (or use --template)"}
	}

	parsed := make([]parsedRecord, 0, len(flagApplyRecords))
	for _, spec := range flagApplyRecords {
		pr, err := parseRecordSpec(spec)
		if err != nil {
			return &UserError{Code: "INVALID_RECORD_SPEC", Msg: err.Error()}
		}
		parsed = append(parsed, pr)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), flagTimeout)
	defer cancel()

	// Resolve provider slug.
	slug := flagProvider
	if slug == "" {
		det, err := entree.DetectProvider(ctx, domain)
		if err != nil {
			return &RuntimeError{Code: "NS_LOOKUP_FAILED", Msg: err.Error()}
		}
		if !det.Supported {
			return &UserError{Code: "PROVIDER_NOT_SUPPORTED", Msg: fmt.Sprintf("provider %q not supported; pass --provider", det.Provider)}
		}
		slug = string(det.Provider)
	}

	creds, err := NewCredentialsLoader(flagCredentialsFile).Load(slug)
	if err != nil {
		return err
	}

	provider, err := entree.NewProvider(slug, creds)
	if err != nil {
		return &RuntimeError{Code: "PROVIDER_INIT_FAILED", Msg: err.Error()}
	}

	f := formatterFromCtx(cmd.Context())

	if flagApplyDryRun {
		diffs := make([]applyDiffEntry, 0, len(parsed))
		for _, pr := range parsed {
			existing, err := provider.GetRecords(ctx, domain, pr.Type)
			if err != nil {
				return &RuntimeError{Code: "GET_RECORDS_FAILED", Msg: err.Error()}
			}
			entry := applyDiffEntry{Type: pr.Type, Name: pr.Name, Proposed: pr.Content, Action: "CREATE"}
			for _, r := range existing {
				if r.Name == pr.Name {
					entry.Current = r.Content
					if r.Content == pr.Content {
						entry.Action = "SKIP"
					} else {
						entry.Action = "UPDATE"
					}
					break
				}
			}
			diffs = append(diffs, entry)
		}
		if f.Mode == ModeHuman {
			for _, d := range diffs {
				fmt.Fprintf(f.Out, "%s %s %s -> %s [%s]\n", d.Type, d.Name, d.Current, d.Proposed, d.Action)
			}
			return nil
		}
		return f.EmitOK(applyDiffData{Records: diffs})
	}

	// Write path.
	if err := RequireYes(IsTTY(os.Stdout), flagYes); err != nil {
		return err
	}

	push := entree.NewPushService(provider)
	results := make([]applyResultEntry, 0, len(parsed))
	anyFailed := false
	for _, pr := range parsed {
		entry := applyResultEntry{Type: pr.Type, Name: pr.Name, RecordValue: pr.Content}
		var res *entree.PushResult
		var perr error
		switch pr.Type {
		case "TXT":
			res, perr = push.PushTXTRecord(ctx, domain, pr.Name, pr.Content)
		case "CNAME":
			res, perr = push.PushCNAMERecord(ctx, domain, pr.Name, pr.Content)
		default:
			// Generic path (A/AAAA/MX/SRV). No idempotency helper -- call SetRecord directly.
			perr = provider.SetRecord(ctx, domain, entree.Record{
				Type: pr.Type, Name: pr.Name, Content: pr.Content, TTL: 300,
			})
			if perr == nil {
				res = &entree.PushResult{Status: entree.StatusCreated, RecordName: pr.Name, RecordValue: pr.Content}
			}
		}
		if perr != nil {
			entry.Status = string(entree.StatusFailed)
			entry.Error = perr.Error()
			anyFailed = true
		} else {
			entry.Status = string(res.Status)
			entry.PreviousValue = res.PreviousValue
			entry.Verified = res.Verified
			if res.VerifyError != nil {
				entry.VerifyError = res.VerifyError.Error()
			}
		}
		results = append(results, entry)
	}

	if f.Mode == ModeHuman {
		for _, r := range results {
			fmt.Fprintf(f.Out, "%s %s %s verified=%v\n", r.Type, r.Name, r.Status, r.Verified)
		}
	} else {
		if err := f.EmitOK(applyResultData{Results: results}); err != nil {
			return err
		}
	}
	if anyFailed {
		return &RuntimeError{Code: "APPLY_PARTIAL_FAILURE", Msg: "one or more records failed to apply"}
	}
	return nil
}

func init() {
	applyCmd.Flags().StringArrayVar(&flagApplyRecords, "record", nil, "record spec TYPE:NAME:VALUE (repeatable)")
	applyCmd.Flags().BoolVar(&flagApplyDryRun, "dry-run", false, "compute diff without writing")
	applyCmd.Flags().StringVar(&flagApplyTemplate, "template", "", "Domain Connect template providerId/serviceId (05-03)")
	applyCmd.Flags().StringArrayVar(&flagApplyVars, "var", nil, "template variable key=value (repeatable)")
	applyCmd.Flags().StringVar(&flagApplyVarsFile, "vars-file", "", "template variables JSON file")
	rootCmd.AddCommand(applyCmd)
}
