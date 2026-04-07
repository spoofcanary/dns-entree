package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type ctxKey string

const (
	ctxKeyRequestID ctxKey = "request_id"
	ctxKeyCredsCtx  ctxKey = "credentials"
	ctxKeyRouteTpl  ctxKey = "route_template"
)

func defaultRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fall back to a timestamp; never panic in middleware.
		return "reqid-fallback"
	}
	return hex.EncodeToString(b[:])
}

// statusRecorder captures the response status for the slog middleware and
// metrics observer. It also tracks bytes written for completeness.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(s int) {
	if r.wroteHeader {
		return
	}
	r.status = s
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(s)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.status = http.StatusOK
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(b)
}

// recoverMW catches panics, logs them, and emits a 500 INTERNAL envelope.
func recoverMW(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
						"method", r.Method,
					)
					writeError(w, http.StatusInternalServerError, CodeInternal, "internal server error", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// requestIDMW assigns a request ID (echoes inbound X-Request-ID if present).
func requestIDMW(gen func() string) func(http.Handler) http.Handler {
	if gen == nil {
		gen = defaultRequestID
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = gen()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// slogMW logs every request with method/path/status/duration/request_id.
// It NEVER inspects request headers (D-25). Defense in depth is layered on by
// credentialRedactMW which strips X-Entree-* headers before slogMW sees the
// request.
func slogMW(logger *slog.Logger, now func() time.Time, mtr *metrics, mux *http.ServeMux) func(http.Handler) http.Handler {
	if now == nil {
		now = time.Now
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			// Resolve the matched route pattern up front so we can use it as
			// a bounded metrics label even for endpoints that have not been
			// registered yet (returns "" -> "unmatched").
			route := ""
			if mux != nil {
				_, route = mux.Handler(r)
			}
			if route == "" {
				route = "unmatched"
			}
			next.ServeHTTP(rec, r)
			dur := now().Sub(start)
			reqID, _ := r.Context().Value(ctxKeyRequestID).(string)
			logger.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"route", route,
				"status", rec.status,
				"duration_ms", dur.Milliseconds(),
				"request_id", reqID,
				"remote_addr", r.RemoteAddr,
			)
			if mtr != nil {
				mtr.observeHTTP(r.Method, route, rec.status, dur)
			}
		})
	}
}

// corsMW emits Access-Control-* headers when origins are configured. With an
// empty allowlist (the default), it is a no-op.
func corsMW(origins []string) func(http.Handler) http.Handler {
	if len(origins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	allowAny := false
	allow := map[string]struct{}{}
	for _, o := range origins {
		if o == "*" {
			allowAny = true
		}
		allow[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowAny {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := allow[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers",
					"Content-Type, X-Request-ID, X-Entree-Provider, "+
						"X-Entree-Cloudflare-Token, X-Entree-AWS-Access-Key-Id, "+
						"X-Entree-AWS-Secret-Access-Key, X-Entree-AWS-Region, "+
						"X-Entree-GoDaddy-Key, X-Entree-GoDaddy-Secret, "+
						"X-Entree-GCDNS-Service-Account-JSON, X-Entree-GCDNS-Project-Id")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// credentialRedactMW stashes the original request headers in the context for
// downstream handlers, then hands the inner chain a *clone* of the request
// with all X-Entree-* headers removed. Logging middleware therefore cannot
// observe credential values even by accident, and a future ReplaceAttr in the
// slog handler is a third layer of defense.
func credentialRedactMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Save originals so handlers can still read credentials.
		originals := make(http.Header, len(credentialHeaderNames))
		scrubbed := r.Header.Clone()
		for name := range credentialHeaderNames {
			if v, ok := r.Header[name]; ok {
				originals[name] = append([]string(nil), v...)
				scrubbed.Del(name)
			}
		}
		ctx := context.WithValue(r.Context(), ctxKeyCredsCtx, originals)
		r2 := r.Clone(ctx)
		r2.Header = scrubbed
		// Re-attach originals via a header that downstream handlers know to
		// fetch only via originalCredentialHeader; we keep them off r2.Header
		// entirely so any caller that touches r2.Header is safe.
		next.ServeHTTP(w, r2)
	})
}

// originalCredentialHeader returns the value of an X-Entree-* header as it
// arrived on the wire, before credentialRedactMW scrubbed it from the request.
// Handlers MUST use this helper rather than r.Header.Get for credential reads.
func originalCredentialHeader(r *http.Request, name string) string {
	canon := http.CanonicalHeaderKey(name)
	hdr, ok := r.Context().Value(ctxKeyCredsCtx).(http.Header)
	if !ok {
		return ""
	}
	if vals, ok := hdr[canon]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}
