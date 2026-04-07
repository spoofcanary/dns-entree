package main

import "github.com/spf13/cobra"

// Build-time injectable via -ldflags "-X main.Version=... -X main.Commit=... -X main.Date=...".
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print entree version",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := formatterFromCtx(cmd.Context())
		return f.EmitOK(map[string]any{
			"version": Version,
			"commit":  Commit,
			"date":    Date,
		})
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
