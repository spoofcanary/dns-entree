package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spoofcanary/dns-entree/migrate"
)

// Server is a stateless HTTP facade over the dns-entree library. It owns the
// mux, middleware chain, metrics registry, and graceful-shutdown plumbing.
// Wave-2 plans register additional routes via RegisterRoutes before Run.
type Server struct {
	opts    Options
	logger  *slog.Logger
	metrics *metrics
	mux     *http.ServeMux

	// httpServer is set by ListenAndServe and read by Shutdown. Guarded by
	// the lifecycle of Run; not safe for concurrent Run calls.
	httpServer *http.Server

	// Stateful migration wiring (Phase 07-04/05). These mirror Options
	// fields after NewServer has resolved defaults; the handlers read them
	// via s.migrationStore etc.
	migrationStore         migrate.MigrationStore
	migrationKey           []byte
	migrationTTL           time.Duration
	migrationRatePerSecond float64

	// sweeper lifecycle
	sweeperStop chan struct{}
	sweeperDone chan struct{}
}

// NewServer constructs a Server, applies defaults, builds the mux with the
// wave-1 endpoints (/healthz, /readyz, /metrics, /v1/openapi.yaml), and
// returns it. Wave-2 plans add business endpoints via RegisterRoutes.
func NewServer(opts Options) *Server {
	if opts.Listen == "" {
		opts.Listen = ":8080"
	}
	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = 15 * time.Minute
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.NewRequestID == nil {
		opts.NewRequestID = defaultRequestID
	}
	if opts.Logger == nil {
		opts.Logger = defaultLogger(opts.LogLevel, opts.LogFormat, os.Stderr)
	}
	if opts.MigrationTTL == 0 {
		opts.MigrationTTL = time.Hour
	}
	if opts.MigrationGCInterval == 0 {
		opts.MigrationGCInterval = 5 * time.Minute
	}
	if opts.MigrationRatePerSecond == 0 {
		opts.MigrationRatePerSecond = 10
	}
	if opts.StateDir == "" {
		opts.StateDir = defaultStateDir()
	}
	if opts.MigrationStore == nil && opts.StateDir != "" {
		js, err := migrate.NewJSONStore(opts.StateDir)
		if err != nil {
			opts.Logger.Warn("migration JSON store init failed; stateful endpoints disabled", "error", err, "state_dir", opts.StateDir)
		} else {
			opts.MigrationStore = js
		}
	}
	if opts.MigrationKey == nil {
		k, fromEnv, err := migrate.LoadStateKey(os.Getenv)
		if err != nil {
			opts.Logger.Warn("ENTREE_STATE_KEY invalid; falling back to derived key", "error", err)
		}
		if fromEnv && k != nil {
			opts.MigrationKey = k
		} else if opts.StateDir != "" {
			derived, derr := migrate.DeriveStateKey(opts.StateDir)
			if derr != nil {
				opts.Logger.Warn("state key derivation failed; stateful endpoints disabled", "error", derr)
			} else {
				opts.MigrationKey = derived
				opts.Logger.Warn("derived state key from hostname+salt; set ENTREE_STATE_KEY for production")
			}
		}
	}

	s := &Server{
		opts:                   opts,
		logger:                 opts.Logger,
		metrics:                newMetrics(),
		mux:                    http.NewServeMux(),
		migrationStore:         opts.MigrationStore,
		migrationKey:           opts.MigrationKey,
		migrationTTL:           opts.MigrationTTL,
		migrationRatePerSecond: opts.MigrationRatePerSecond,
	}
	s.registerWave1Routes()
	registerCoreRoutes(s)
	registerExtraRoutes(s)
	return s
}

// defaultStateDir returns the XDG default path for migration state files.
func defaultStateDir() string {
	if v := os.Getenv("ENTREE_STATE_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "entree", "migrations")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "share", "entree", "migrations")
	}
	return filepath.Join(os.TempDir(), "entree", "migrations")
}

func defaultLogger(level, format string, w io.Writer) *slog.Logger {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lv}
	if format == "text" {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

// Mux exposes the underlying ServeMux so wave-2 plans (06-02, 06-03) can
// register additional method+path patterns directly. Routes registered after
// Handler() is built are still served because the mux is referenced by
// pointer through the chain.
func (s *Server) Mux() *http.ServeMux { return s.mux }

// Metrics exposes the registry so handlers in wave-2 can call IncProviderOp.
func (s *Server) Metrics() *metrics { return s.metrics }

// Logger returns the configured slog logger.
func (s *Server) Logger() *slog.Logger { return s.logger }

// Options returns a copy of the resolved Options (for test introspection).
func (s *Server) Options() Options { return s.opts }

// RegisterRoutes runs the supplied callback against the underlying ServeMux.
// Wave-2 plans use this to attach handlers without poking at internals.
func (s *Server) RegisterRoutes(fn func(mux *http.ServeMux)) {
	if fn != nil {
		fn(s.mux)
	}
}

// Handler returns the fully wrapped http.Handler. Composition order
// outermost-first: recover -> requestID -> credentialRedact -> slog -> cors
// -> mux. credentialRedact runs BEFORE slog so logging middleware cannot see
// raw credential headers (D-25). slogMW resolves the matched route pattern
// from the mux up front so it can be used as the metrics label without
// inflating cardinality on path variations.
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	h = corsMW(s.opts.CORSOrigins)(h)
	h = slogMW(s.logger, s.opts.Now, s.metrics, s.mux)(h)
	h = credentialRedactMW(h)
	h = requestIDMW(s.opts.NewRequestID)(h)
	h = recoverMW(s.logger)(h)
	return h
}

// ListenAndServe binds to opts.Listen and serves until ctx is cancelled or
// the listener fails. On ctx cancel, it triggers a 30-second graceful drain.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.opts.Listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      s.opts.RequestTimeout,
		IdleTimeout:       120 * time.Second,
	}
	s.httpServer = srv
	s.startSweeper()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Shutdown drains in-flight requests and closes the listener.
func (s *Server) Shutdown(ctx context.Context) error {
	s.stopSweeper()
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// startSweeper launches the background GC goroutine. Safe to call multiple
// times; second call is a no-op.
func (s *Server) startSweeper() {
	if s.sweeperStop != nil {
		return
	}
	if s.migrationStore == nil || s.opts.MigrationGCInterval <= 0 {
		return
	}
	s.sweeperStop = make(chan struct{})
	s.sweeperDone = make(chan struct{})
	interval := s.opts.MigrationGCInterval
	store := s.migrationStore
	logger := s.logger
	go func() {
		defer close(s.sweeperDone)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.sweeperStop:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				n, err := store.SweepExpired(ctx, time.Now().UTC())
				cancel()
				if err != nil {
					logger.Warn("migration sweep failed", "error", err)
					continue
				}
				if n > 0 {
					logger.Info("migration sweep removed expired rows", "count", n)
				}
			}
		}
	}()
}

// stopSweeper signals the sweeper to exit and waits for it. Idempotent.
func (s *Server) stopSweeper() {
	if s.sweeperStop == nil {
		return
	}
	select {
	case <-s.sweeperStop:
		// already closed
	default:
		close(s.sweeperStop)
	}
	if s.sweeperDone != nil {
		select {
		case <-s.sweeperDone:
		case <-time.After(2 * time.Second):
		}
	}
	s.sweeperStop = nil
	s.sweeperDone = nil
}

// MaxBytesHelper wraps r.Body in an http.MaxBytesReader. Wave-2 handlers call
// this with BodyLimitDefault (1 MiB) for normal endpoints and BodyLimitLarge
// (10 MiB) for zone/migrate per the threat model T-06-03.
func MaxBytesHelper(w http.ResponseWriter, r *http.Request, limit int64) io.ReadCloser {
	return http.MaxBytesReader(w, r.Body, limit)
}

// ----- wave-1 routes ---------------------------------------------------------

func (s *Server) registerWave1Routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.HandleFunc("GET /metrics", s.metrics.handler)
	s.mux.HandleFunc("GET /v1/openapi.yaml", openapiHandler)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.opts.ReadyCheck != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := s.opts.ReadyCheck(ctx); err != nil {
			s.logger.Warn("readyz failed", "error", err)
			writeError(w, http.StatusServiceUnavailable, CodeInternal, "not ready", nil)
			return
		}
	}
	writeJSON(w, map[string]any{"ok": true})
}

// jsonOK and jsonErr are convenience aliases re-exported as package locals so
// wave-2 handlers can stay terse. They simply call writeJSON / writeError.
var (
	_ = json.Marshal // keep encoding/json imported for wave-2 handlers
)
