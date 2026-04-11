package main

import (
	"fmt"

	"github.com/spf13/cobra"
	entree "github.com/spoofcanary/dns-entree"
)

type spfMergeData struct {
	Value               string   `json:"value"`
	Changed             bool     `json:"changed"`
	BrokenInput         bool     `json:"broken_input"`
	LookupCount         int      `json:"lookup_count"`
	LookupLimitExceeded bool     `json:"lookup_limit_exceeded"`
	Warnings            []string `json:"warnings"`
}

var spfMergeCmd = &cobra.Command{
	Use:   "spf-merge <current> <include> [<include>...]",
	Short: "Merge SPF includes into an existing record (pure, no I/O)",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		current := args[0]
		includes := args[1:]
		res, err := entree.MergeSPF(current, includes)
		if err != nil {
			return &RuntimeError{Code: "SPF_MERGE_FAILED", Msg: err.Error()}
		}
		f := formatterFromCtx(cmd.Context())
		data := spfMergeData{
			Value:               res.Value,
			Changed:             res.Changed,
			BrokenInput:         res.BrokenInput,
			LookupCount:         res.LookupCount,
			LookupLimitExceeded: res.LookupLimitExceeded,
			Warnings:            res.Warnings,
		}
		if f.Mode == ModeHuman {
			fmt.Fprintln(f.Out, data.Value)
			for _, w := range data.Warnings {
				fmt.Fprintf(f.Err, "warning: %s\n", w)
			}
			return nil
		}
		return f.EmitOK(data)
	},
}

func init() {
	rootCmd.AddCommand(spfMergeCmd)
}
