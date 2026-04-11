package api

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Env var names. Documented in docs/http-api.md.
const (
	EnvListen           = "ENTREE_API_LISTEN"
	EnvLogLevel         = "ENTREE_API_LOG_LEVEL"
	EnvLogFormat        = "ENTREE_API_LOG_FORMAT"
	EnvCORSOrigin       = "ENTREE_API_CORS_ORIGIN"
	EnvRequestTimeout   = "ENTREE_API_REQUEST_TIMEOUT"
	EnvTemplateCacheDir = "ENTREE_API_TEMPLATE_CACHE_DIR"
	EnvStateDir         = "ENTREE_API_STATE_DIR"
	EnvMigrationTTL     = "ENTREE_API_MIGRATION_TTL"
	EnvGCInterval       = "ENTREE_API_GC_INTERVAL"
	EnvMigrationRate    = "ENTREE_API_MIGRATION_RATE"
)

// corsList is a flag.Value that allows --cors-origin to be passed multiple
// times. Each value is appended to the underlying slice. The user-set bit is
// tracked so env-var values can be ignored when the flag was supplied.
type corsList struct {
	values []string
	set    bool
}

func (c *corsList) String() string {
	if c == nil {
		return ""
	}
	return strings.Join(c.values, ",")
}

func (c *corsList) Set(v string) error {
	if v == "" {
		return nil
	}
	c.values = append(c.values, v)
	c.set = true
	return nil
}

// flagSetState tracks which flags were explicitly supplied so LoadFromEnv can
// honor D-27: flags take precedence over env vars when set.
type flagSetState struct {
	listen           bool
	logLevel         bool
	logFormat        bool
	requestTimeout   bool
	templateCacheDir bool
	stateDir         bool
	migrationTTL     bool
	gcInterval       bool
	migrationRate    bool
	cors             *corsList
}

// FlagBindings groups the resolved Options pointer and the per-flag set bits
// returned by BindFlags. Pass it to LoadFromEnv after fs.Parse.
type FlagBindings struct {
	Opts  *Options
	state *flagSetState
}

// BindFlags registers the standard --listen / --log-level / --log-format /
// --cors-origin / --request-timeout / --template-cache-dir flags onto fs and
// returns a FlagBindings handle. Both cmd/entree-api and `entree serve` call
// this so the flag set is identical (D-01, D-27).
//
// Defaults match D-27: listen :8080, log-level info, log-format json,
// request-timeout 15m, cors empty, template cache dir XDG default.
func BindFlags(fs *flag.FlagSet) *FlagBindings {
	opts := &Options{
		Listen:                 ":8080",
		LogLevel:               "info",
		LogFormat:              "json",
		RequestTimeout:         15 * time.Minute,
		TemplateCacheDir:       defaultTemplateCacheDir(),
		StateDir:               defaultStateDirFlag(),
		MigrationTTL:           time.Hour,
		MigrationGCInterval:    5 * time.Minute,
		MigrationRatePerSecond: 10,
	}
	state := &flagSetState{cors: &corsList{}}

	fs.StringVar(&opts.Listen, "listen", opts.Listen, "bind address (host:port)")
	fs.StringVar(&opts.LogLevel, "log-level", opts.LogLevel, "log level: debug|info|warn|error")
	fs.StringVar(&opts.LogFormat, "log-format", opts.LogFormat, "log format: json|text")
	fs.Var(state.cors, "cors-origin", "CORS allowlist origin (repeatable; pass '*' for any)")
	fs.DurationVar(&opts.RequestTimeout, "request-timeout", opts.RequestTimeout, "max request duration")
	fs.StringVar(&opts.TemplateCacheDir, "template-cache-dir", opts.TemplateCacheDir, "template cache directory")
	fs.StringVar(&opts.StateDir, "state-dir", opts.StateDir, "migration state directory (JSON store)")
	fs.DurationVar(&opts.MigrationTTL, "migration-ttl", opts.MigrationTTL, "migration row TTL before GC (e.g. 1h)")
	fs.DurationVar(&opts.MigrationGCInterval, "gc-interval", opts.MigrationGCInterval, "background sweeper interval")
	fs.Float64Var(&opts.MigrationRatePerSecond, "migration-rate", opts.MigrationRatePerSecond, "apply records/sec rate limit")

	// Track which flags were explicitly set via fs.Visit after parsing - we
	// can't override fs.StringVar's setter, so the trick is to record the
	// state lazily in LoadFromEnv (which inspects fs.Visit).
	return &FlagBindings{Opts: opts, state: state}
}

// LoadFromEnv overlays ENTREE_API_* environment variables onto opts using
// D-27 precedence: flags win when explicitly set, env wins when the flag is
// at its default. fs is the same FlagSet that BindFlags was called against;
// it must have already been parsed. env is typically os.Getenv but is
// injectable for tests.
func LoadFromEnv(fs *flag.FlagSet, b *FlagBindings, env func(string) string) error {
	if env == nil {
		env = os.Getenv
	}
	if b == nil || b.Opts == nil || b.state == nil {
		return fmt.Errorf("options_flags: nil bindings")
	}

	// Mark which flags were explicitly set by walking fs.Visit (only visited
	// flags are ones the user supplied). This must run after fs.Parse.
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "listen":
			b.state.listen = true
		case "log-level":
			b.state.logLevel = true
		case "log-format":
			b.state.logFormat = true
		case "request-timeout":
			b.state.requestTimeout = true
		case "template-cache-dir":
			b.state.templateCacheDir = true
		case "cors-origin":
			b.state.cors.set = true
		case "state-dir":
			b.state.stateDir = true
		case "migration-ttl":
			b.state.migrationTTL = true
		case "gc-interval":
			b.state.gcInterval = true
		case "migration-rate":
			b.state.migrationRate = true
		}
	})

	if !b.state.listen {
		if v := env(EnvListen); v != "" {
			b.Opts.Listen = v
		}
	}
	if !b.state.logLevel {
		if v := env(EnvLogLevel); v != "" {
			b.Opts.LogLevel = v
		}
	}
	if !b.state.logFormat {
		if v := env(EnvLogFormat); v != "" {
			b.Opts.LogFormat = v
		}
	}
	if !b.state.requestTimeout {
		if v := env(EnvRequestTimeout); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid %s: %w", EnvRequestTimeout, err)
			}
			b.Opts.RequestTimeout = d
		}
	}
	if !b.state.templateCacheDir {
		if v := env(EnvTemplateCacheDir); v != "" {
			b.Opts.TemplateCacheDir = v
		}
	}
	if !b.state.cors.set {
		if v := env(EnvCORSOrigin); v != "" {
			parts := strings.Split(v, ",")
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					out = append(out, p)
				}
			}
			b.Opts.CORSOrigins = out
		}
	} else {
		b.Opts.CORSOrigins = append([]string(nil), b.state.cors.values...)
	}
	if !b.state.stateDir {
		if v := env(EnvStateDir); v != "" {
			b.Opts.StateDir = v
		}
	}
	if !b.state.migrationTTL {
		if v := env(EnvMigrationTTL); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid %s: %w", EnvMigrationTTL, err)
			}
			b.Opts.MigrationTTL = d
		}
	}
	if !b.state.gcInterval {
		if v := env(EnvGCInterval); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid %s: %w", EnvGCInterval, err)
			}
			b.Opts.MigrationGCInterval = d
		}
	}
	if !b.state.migrationRate {
		if v := env(EnvMigrationRate); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("invalid %s: %w", EnvMigrationRate, err)
			}
			b.Opts.MigrationRatePerSecond = f
		}
	}

	return nil
}

// defaultStateDirFlag returns the migration state dir default for the CLI
// flag (mirrors api/server.go defaultStateDir which is used by NewServer).
func defaultStateDirFlag() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "entree", "migrations")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "share", "entree", "migrations")
	}
	return filepath.Join(os.TempDir(), "entree", "migrations")
}

func defaultTemplateCacheDir() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "dns-entree", "templates")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "dns-entree", "templates")
	}
	return filepath.Join(os.TempDir(), "dns-entree", "templates")
}
