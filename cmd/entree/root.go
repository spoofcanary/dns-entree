package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
)

type ctxKey string

const (
	ctxKeyFormatter ctxKey = "formatter"
	ctxKeyLogger    ctxKey = "logger"
)

// activeFormatter is set in PersistentPreRunE so Execute() can emit structured
// errors even when cobra's context propagation differs between root and sub.
var activeFormatter *Formatter

// Global flag values.
var (
	flagJSON            bool
	flagQuiet           bool
	flagLogLevel        string
	flagNoColor         bool
	flagCredentialsFile string
	flagProvider        string
	flagYes             bool
	flagTimeout         time.Duration
)

var rootCmd = &cobra.Command{
	Use:           "entree",
	Short:         "DNS provider CLI (agent-friendly)",
	Long:          "entree is a DNS provider CLI for detection, record push, verification, SPF merge, and Domain Connect templates.\n\nExit codes: 0 success, 1 runtime error, 2 user error.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		mode, err := ParseMode(flagJSON, flagQuiet)
		if err != nil {
			return &UserError{Code: "MUTUALLY_EXCLUSIVE", Msg: err.Error()}
		}
		level := parseLogLevel(flagLogLevel)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
		f := &Formatter{Mode: mode, Out: os.Stdout, Err: os.Stderr}
		activeFormatter = f
		ctx := context.WithValue(cmd.Context(), ctxKeyFormatter, f)
		ctx = context.WithValue(ctx, ctxKeyLogger, logger)
		cmd.SetContext(ctx)
		return nil
	},
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.BoolVar(&flagJSON, "json", false, "emit machine-readable JSON output")
	pf.BoolVar(&flagQuiet, "quiet", false, "suppress non-error output (exit code only)")
	pf.StringVar(&flagLogLevel, "log-level", "warn", "log level: debug|info|warn|error|off")
	pf.BoolVar(&flagNoColor, "no-color", false, "disable color in human output")
	pf.StringVar(&flagCredentialsFile, "credentials-file", "", "path to credentials JSON file")
	pf.StringVar(&flagProvider, "provider", "", "force DNS provider (cloudflare|route53|godaddy|google_cloud_dns)")
	pf.BoolVar(&flagYes, "yes", false, "confirm destructive operations (required under non-TTY)")
	pf.DurationVar(&flagTimeout, "timeout", 30*time.Second, "operation timeout")
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "off":
		return slog.Level(127)
	default:
		return slog.LevelWarn
	}
}

func formatterFromCtx(ctx context.Context) *Formatter {
	if f, ok := ctx.Value(ctxKeyFormatter).(*Formatter); ok && f != nil {
		return f
	}
	return &Formatter{Mode: ModeHuman, Out: os.Stdout, Err: os.Stderr}
}

func loggerFromCtx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

// Execute runs the root command and returns a stable exit code.
func Execute() int {
	err := rootCmd.ExecuteContext(context.Background())
	if err == nil {
		return ExitOK
	}
	f := activeFormatter
	if f == nil {
		f = formatterFromCtx(rootCmd.Context())
	}
	code, msg, details := classifyForOutput(err)
	_ = f.EmitError(code, msg, details)
	return ClassifyExit(err)
}

func classifyForOutput(err error) (string, string, map[string]any) {
	var ue *UserError
	if errors.As(err, &ue) {
		return nonEmpty(ue.Code, "USER_ERROR"), ue.Msg, ue.Details
	}
	var re *RuntimeError
	if errors.As(err, &re) {
		return nonEmpty(re.Code, "RUNTIME_ERROR"), re.Msg, re.Details
	}
	return "RUNTIME_ERROR", fmt.Sprint(err), nil
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
