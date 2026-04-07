package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spoofcanary/dns-entree/domainconnect"
)

// dcDiscoverResult is the stable JSON shape for `entree dc-discover`.
type dcDiscoverResult struct {
	Domain       string   `json:"domain"`
	Supported    bool     `json:"supported"`
	ProviderID   string   `json:"provider_id,omitempty"`
	ProviderName string   `json:"provider_name,omitempty"`
	URLSyncUX    string   `json:"url_sync_ux,omitempty"`
	URLAsyncUX   string   `json:"url_async_ux,omitempty"`
	URLAPI       string   `json:"url_api,omitempty"`
	Width        int      `json:"width,omitempty"`
	Height       int      `json:"height,omitempty"`
	Nameservers  []string `json:"nameservers,omitempty"`
}

// discoverFn is the discovery seam (swapped in tests).
var discoverFn = func(ctx context.Context, domain string) (domainconnect.DiscoveryResult, error) {
	return domainconnect.Discover(ctx, domain)
}

var dcDiscoverCmd = &cobra.Command{
	Use:   "dc-discover <domain>",
	Short: "Probe a domain for Domain Connect v2 support",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := strings.TrimSpace(args[0])
		if domain == "" {
			return &UserError{Code: "INVALID_DOMAIN", Msg: "domain must not be empty"}
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), flagTimeout)
		defer cancel()

		res, err := discoverFn(ctx, domain)
		if err != nil {
			return &UserError{Code: "INVALID_DOMAIN", Msg: err.Error()}
		}

		out := dcDiscoverResult{
			Domain:       domain,
			Supported:    res.Supported,
			ProviderID:   res.ProviderID,
			ProviderName: res.ProviderName,
			URLSyncUX:    res.URLSyncUX,
			URLAsyncUX:   res.URLAsyncUX,
			URLAPI:       res.URLAPI,
			Width:        res.Width,
			Height:       res.Height,
			Nameservers:  res.Nameservers,
		}
		return formatterFromCtx(cmd.Context()).EmitOK(out)
	},
}

func init() {
	rootCmd.AddCommand(dcDiscoverCmd)
}
