package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	entree "github.com/spoofcanary/dns-entree"
)

type detectData struct {
	Provider    string   `json:"provider"`
	Label       string   `json:"label"`
	Supported   bool     `json:"supported"`
	Nameservers []string `json:"nameservers"`
	Method      string   `json:"method"`
}

var detectCmd = &cobra.Command{
	Use:   "detect <domain>",
	Short: "Detect the DNS hosting provider for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]
		if domain == "" {
			return &UserError{Code: "INVALID_DOMAIN", Msg: "domain required"}
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), flagTimeout)
		defer cancel()

		res, err := entree.DetectProvider(ctx, domain)
		if err != nil {
			return &RuntimeError{Code: "NS_LOOKUP_FAILED", Msg: err.Error()}
		}
		f := formatterFromCtx(cmd.Context())
		data := detectData{
			Provider:    string(res.Provider),
			Label:       res.Label,
			Supported:   res.Supported,
			Nameservers: res.Nameservers,
			Method:      res.Method,
		}
		if f.Mode == ModeHuman {
			fmt.Fprintf(f.Out, "provider=%s label=%q supported=%v method=%s\n",
				data.Provider, data.Label, data.Supported, data.Method)
			return nil
		}
		return f.EmitOK(data)
	},
}

func init() {
	rootCmd.AddCommand(detectCmd)
}
