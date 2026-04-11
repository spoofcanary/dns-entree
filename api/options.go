package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/spoofcanary/dns-entree/migrate"
)

// Options configures a Server. Zero values get sensible defaults from
// NewServer; only Listen is required when calling ListenAndServe.
type Options struct {
	// Listen is the bind address (host:port). Default ":8080".
	Listen string

	// LogLevel selects slog level. Empty string => info.
	LogLevel string

	// LogFormat selects "json" or "text". Empty => json.
	LogFormat string

	// CORSOrigins is the allowlist for the CORS middleware. Empty disables
	// CORS entirely.
	CORSOrigins []string

	// RequestTimeout is the http.Server WriteTimeout. Default 15m (D-23).
	RequestTimeout time.Duration

	// TemplateCacheDir is forwarded to wave-2 template handlers.
	TemplateCacheDir string

	// ReadyCheck, if non-nil, is invoked by /readyz. A nil error means ready.
	ReadyCheck func(context.Context) error

	// Logger overrides the default slog logger. Tests inject a buffer-backed
	// JSON handler here to assert credential redaction.
	Logger *slog.Logger

	// Now overrides time.Now (test seam for deterministic durations).
	Now func() time.Time

	// NewRequestID overrides the default crypto/rand request ID generator.
	NewRequestID func() string

	// StateDir is the directory for stateful migration JSON files. Default
	// $XDG_DATA_HOME/entree/migrations (falls back to ~/.local/share/...).
	StateDir string

	// MigrationStore is the backend for stateful migrations. If nil and
	// StateDir is non-empty, NewServer constructs a JSON-backed store.
	MigrationStore migrate.MigrationStore

	// MigrationKey is the AES-256 key used to seal credential blobs at rest.
	// If nil, NewServer resolves via LoadStateKey(os.Getenv) with a
	// DeriveStateKey(StateDir) fallback (WARN logged).
	MigrationKey []byte

	// MigrationTTL is the lifetime of a migration row. Default 1h (D-11).
	MigrationTTL time.Duration

	// MigrationGCInterval is the sweeper tick period. Default 5m (D-16).
	MigrationGCInterval time.Duration

	// MigrationRatePerSecond caps per-record apply writes. Default 10.
	MigrationRatePerSecond float64
}

// Body size limits exported for wave-2 handlers (T-06-03).
const (
	BodyLimitDefault int64 = 1 << 20  // 1 MiB
	BodyLimitLarge   int64 = 10 << 20 // 10 MiB - zone import / migrate
)
