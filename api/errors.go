package api

import (
	"encoding/json"
	"net/http"
)

// SchemaVersion is the stable JSON envelope schema version. Mirrors the CLI
// formatter (cmd/entree/output.go) so a single client parser handles both.
const SchemaVersion = 1

// Stable error codes. Documented in api/openapi.yaml; never rename without a
// schema version bump.
const (
	CodeMissingCredentials = "MISSING_CREDENTIALS"
	CodeBadRequest         = "BAD_REQUEST"
	CodeProviderError      = "PROVIDER_ERROR"
	CodeVerifyMismatch     = "VERIFY_MISMATCH"
	CodeTemplateNotFound   = "TEMPLATE_NOT_FOUND"
	CodeRateLimit          = "RATE_LIMIT_EXCEEDED"
	CodeInternal           = "INTERNAL"
)

// okEnvelope and errEnvelope are intentionally duplicated from
// cmd/entree/output.go (which is package main and cannot be imported). The
// drift_test.go test asserts byte-identical JSON output to prevent silent
// drift between the CLI and API contracts.
type okEnvelope struct {
	OK            bool `json:"ok"`
	SchemaVersion int  `json:"schema_version"`
	Data          any  `json:"data"`
}

type errEnvelope struct {
	OK            bool       `json:"ok"`
	SchemaVersion int        `json:"schema_version"`
	Error         errPayload `json:"error"`
}

type errPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// sensitiveDetailKeys are stripped from error details before serialisation as
// defense in depth (D-26). Handlers should never put credential values in
// details to begin with; this catches accidents.
var sensitiveDetailKeys = map[string]struct{}{
	"value":    {},
	"token":    {},
	"secret":   {},
	"password": {},
	"key":      {},
}

func scrubDetails(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if _, drop := sensitiveDetailKeys[k]; drop {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// writeJSON writes a success envelope at HTTP 200.
func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(okEnvelope{OK: true, SchemaVersion: SchemaVersion, Data: data})
}

// writeError writes a structured error envelope at the supplied HTTP status.
// Detail keys that look like credentials are stripped (D-26).
func writeError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(errEnvelope{
		OK:            false,
		SchemaVersion: SchemaVersion,
		Error: errPayload{
			Code:    code,
			Message: message,
			Details: scrubDetails(details),
		},
	})
}
