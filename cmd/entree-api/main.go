// Command entree-api is the standalone HTTP server binary for dns-entree.
//
// Install:
//
//	go install github.com/spoofcanary/dns-entree/cmd/entree-api@latest
//
// Subcommands:
//
//	entree-api [flags]           # default: run the HTTP server (serve)
//	entree-api serve [flags]     # explicit serve
//	entree-api gc --once [flags] # one-shot SweepExpired then exit
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
	"time"

	"github.com/spoofcanary/dns-entree/api"
	"github.com/spoofcanary/dns-entree/migrate"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code := run(ctx, os.Args[1:], os.Getenv, os.Stderr)
	os.Exit(code)
}

// run is the testable entry point. It dispatches to the serve or gc
// subcommand. Unknown first arg falls through to serve for back-compat.
func run(ctx context.Context, args []string, env func(string) string, errOut io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "serve":
			return runServe(ctx, args[1:], env, errOut)
		case "gc":
			return runGC(args[1:], env, errOut)
		case "-h", "--help":
			// Fall through to serve which prints usage via flag parse error.
		}
	}
	return runServe(ctx, args, env, errOut)
}

func runServe(ctx context.Context, args []string, env func(string) string, errOut io.Writer) int {
	fs := flag.NewFlagSet("entree-api", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "entree-api - dns-entree HTTP server")
		fmt.Fprintln(errOut, "")
		fmt.Fprintln(errOut, "Usage:")
		fmt.Fprintln(errOut, "  entree-api [flags]           run the HTTP server")
		fmt.Fprintln(errOut, "  entree-api gc --once [flags] one-shot GC sweep of expired migrations")
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

// runGC handles `entree-api gc --once`: load the JSON migration store from
// --state-dir (or ENTREE_API_STATE_DIR) and call SweepExpired once. Only
// --once is supported; the long-running background sweeper lives inside the
// HTTP server (api.Server).
func runGC(args []string, env func(string) string, errOut io.Writer) int {
	fs := flag.NewFlagSet("entree-api gc", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprintln(errOut, "entree-api gc - one-shot migration GC sweep")
		fmt.Fprintln(errOut, "")
		fmt.Fprintln(errOut, "Usage: entree-api gc --once [--state-dir <dir>]")
		fmt.Fprintln(errOut, "")
		fs.PrintDefaults()
	}
	var once bool
	fs.BoolVar(&once, "once", false, "run a single sweep and exit (required)")
	// Re-use the shared bindings so --state-dir / env fallback behaves the
	// same as the serve path.
	bindings := api.BindFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := api.LoadFromEnv(fs, bindings, env); err != nil {
		fmt.Fprintln(errOut, "config error:", err)
		return 2
	}
	if !once {
		fmt.Fprintln(errOut, "entree-api gc: only --once is supported")
		return 2
	}

	dir := bindings.Opts.StateDir
	if dir == "" {
		fmt.Fprintln(errOut, "entree-api gc: state dir is empty (set --state-dir or ENTREE_API_STATE_DIR)")
		return 2
	}
	store, err := migrate.NewJSONStore(dir)
	if err != nil {
		fmt.Fprintln(errOut, "entree-api gc:", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	n, err := store.SweepExpired(ctx, time.Now())
	if err != nil {
		fmt.Fprintln(errOut, "entree-api gc: sweep:", err)
		return 1
	}
	fmt.Fprintf(errOut, "swept %d expired migrations\n", n)
	return 0
}
