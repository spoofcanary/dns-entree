package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/spoofcanary/dns-entree/api"
)

// serveCmd starts the dns-entree HTTP server using the same flag set and env
// vars as cmd/entree-api. Both entry points share api.BindFlags / api.LoadFromEnv.
var serveCmd = &cobra.Command{
	Use:                "serve",
	Short:              "Run the dns-entree HTTP API server",
	Long:               "Start the dns-entree HTTP API server. Identical flags and env vars as the standalone entree-api binary; see docs/http-api.md.",
	SilenceUsage:       true,
	SilenceErrors:      true,
	DisableFlagParsing: true, // we manage our own FlagSet for parity with cmd/entree-api
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runServe(ctx, args, os.Getenv)
	},
}

func runServe(ctx context.Context, args []string, env func(string) string) error {
	fs := flag.NewFlagSet("entree serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	bindings := api.BindFlags(fs)
	if err := fs.Parse(args); err != nil {
		return &UserError{Code: "BAD_FLAGS", Msg: err.Error()}
	}
	if err := api.LoadFromEnv(fs, bindings, env); err != nil {
		return &UserError{Code: "BAD_CONFIG", Msg: err.Error()}
	}
	srv := api.NewServer(*bindings.Opts)
	srv.Logger().Info("listening", "addr", bindings.Opts.Listen)
	if err := srv.ListenAndServe(ctx); err != nil {
		return &RuntimeError{Code: "SERVER_ERROR", Msg: err.Error()}
	}
	srv.Logger().Info("shutdown complete")
	return nil
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
