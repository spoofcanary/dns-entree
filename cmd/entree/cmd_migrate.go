package main

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagMigrateTo            string
	flagMigrateRate          float64
	flagMigrateDryRun        bool
	flagMigrateNoAXFR        bool
	flagMigrateLabels        []string
	flagMigrateLabelsOnly    []string
	flagMigrateLabelsFile    string
	flagMigrateVerifyTimeout time.Duration
)

var migrateCmd = &cobra.Command{
	Use:   "migrate <domain>",
	Short: "Scrape, apply, and verify a DNS zone against a target provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := strings.TrimSpace(args[0])
		if domain == "" {
			return &UserError{Code: "INVALID_DOMAIN", Msg: "domain required"}
		}
		if flagMigrateTo == "" {
			return &UserError{Code: "MISSING_TO", Msg: "--to <slug> required"}
		}
		if flagMigrateRate < 0 || flagMigrateRate > 100 {
			return &UserError{Code: "INVALID_RATE", Msg: "--rate must be between 0 and 100"}
		}

		labels, err := loadLabelsFile(flagMigrateLabelsFile)
		if err != nil {
			return &UserError{Code: "LABELS_FILE_READ_FAILED", Msg: err.Error()}
		}
		extra := append([]string{}, flagMigrateLabels...)
		extra = append(extra, labels...)

		return runMigrateCore(cmd, domain, flagMigrateTo, migrateCoreOpts{
			dryRun:        flagMigrateDryRun,
			rate:          flagMigrateRate,
			skipAXFR:      flagMigrateNoAXFR,
			scrapeLabels:  extra,
			scrapeOnly:    flagMigrateLabelsOnly,
			verifyTimeout: flagMigrateVerifyTimeout,
		})
	},
}

func init() {
	migrateCmd.Flags().StringVar(&flagMigrateTo, "to", "", "target provider slug (required)")
	migrateCmd.Flags().Float64Var(&flagMigrateRate, "rate", 10, "writes per second rate limit (max 100)")
	migrateCmd.Flags().BoolVar(&flagMigrateDryRun, "dry-run", false, "Phase A only; do not write")
	migrateCmd.Flags().BoolVar(&flagMigrateNoAXFR, "no-axfr", false, "disable AXFR attempt")
	migrateCmd.Flags().StringSliceVar(&flagMigrateLabels, "labels", nil, "additional labels to enumerate")
	migrateCmd.Flags().StringSliceVar(&flagMigrateLabelsOnly, "labels-only", nil, "replace default label list")
	migrateCmd.Flags().StringVar(&flagMigrateLabelsFile, "labels-file", "", "file with extra labels")
	migrateCmd.Flags().DurationVar(&flagMigrateVerifyTimeout, "verify-timeout", 5*time.Minute, "post-apply verification timeout")
	rootCmd.AddCommand(migrateCmd)
}
