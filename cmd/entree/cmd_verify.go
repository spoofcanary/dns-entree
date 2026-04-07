package main

import (
	"context"
	"fmt"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spf13/cobra"
)

type verifyData struct {
	Verified           bool     `json:"verified"`
	CurrentValue       string   `json:"current_value"`
	Method             string   `json:"method"`
	NameserversQueried []string `json:"nameservers_queried"`
}

var flagVerifyContains string

var verifyCmd = &cobra.Command{
	Use:   "verify <domain> <type> <name>",
	Short: "Query authoritative NS for a record and report the value",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain, rtype, name := args[0], args[1], args[2]
		if domain == "" {
			return &UserError{Code: "INVALID_DOMAIN", Msg: "domain required"}
		}
		if !isValidRecordType(rtype) {
			return &UserError{Code: "INVALID_RECORD_TYPE", Msg: fmt.Sprintf("unsupported record type %q", rtype)}
		}
		if name == "" {
			return &UserError{Code: "INVALID_NAME", Msg: "name required"}
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), flagTimeout)
		defer cancel()

		res, err := entree.Verify(ctx, domain, entree.VerifyOpts{
			RecordType: rtype,
			Name:       name,
			Contains:   flagVerifyContains,
		})
		if err != nil {
			return &RuntimeError{Code: "DNS_QUERY_FAILED", Msg: err.Error()}
		}

		f := formatterFromCtx(cmd.Context())
		data := verifyData{
			Verified:           res.Verified,
			CurrentValue:       res.CurrentValue,
			Method:             res.Method,
			NameserversQueried: res.NameserversQueried,
		}
		if f.Mode == ModeHuman {
			fmt.Fprintf(f.Out, "verified=%v value=%q method=%s\n", data.Verified, data.CurrentValue, data.Method)
			return nil
		}
		return f.EmitOK(data)
	},
}

func isValidRecordType(t string) bool {
	switch t {
	case "TXT", "CNAME", "MX", "A", "AAAA":
		return true
	}
	return false
}

func init() {
	verifyCmd.Flags().StringVar(&flagVerifyContains, "contains", "", "case-insensitive substring match on record value")
	rootCmd.AddCommand(verifyCmd)
}
