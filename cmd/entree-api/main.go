// Command entree-api is the standalone HTTP server binary for dns-entree.
//
// Install:
//
//	go install github.com/spoofcanary/dns-entree/cmd/entree-api@latest
//
// See docs/http-api.md for the deployment model and curl examples.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spoofcanary/dns-entree/api"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code := run(ctx, os.Args[1:], os.Getenv, os.Stderr)
	os.Exit(code)
}

// run is the testable entry point. It returns a process exit code and writes
// human diagnostics (not credentials) to errOut. ctx cancellation triggers a
// graceful shutdown via api.Server.ListenAndServe.
func run(ctx context.Context, args []string, env func(string) string, errOut io.Writer) int {
	fs := flag.NewFlagSet("entree-api", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "entree-api - dns-entree HTTP server")
		fmt.Fprintln(errOut, "")
		fmt.Fprintln(errOut, "Usage: entree-api [flags]")
		fmt.Fprintln(errOut, "")
		fs.PrintDefaults()
	}

	bindings := api.BindFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := api.LoadFromEnv(fs, bindings, env); err != nil {
		fmt.Fprintln(errOut, "config error:", err)
		return 2
	}

	srv := api.NewServer(*bindings.Opts)
	srv.Logger().Info("listening", "addr", bindings.Opts.Listen)
	if err := srv.ListenAndServe(ctx); err != nil {
		srv.Logger().Error("server exited", "error", err)
		return 1
	}
	srv.Logger().Info("shutdown complete")
	return 0
}
