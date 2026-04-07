package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// SchemaVersion is the stable JSON envelope schema version (D-P1).
const SchemaVersion = 1

// Mode controls output rendering.
type Mode int

const (
	ModeHuman Mode = iota
	ModeJSON
	ModeQuiet
)

// HumanRenderer lets data types provide their own human-mode rendering.
type HumanRenderer interface {
	RenderHuman(io.Writer) error
}

// Formatter writes command output in the configured mode.
// Out is reserved for command output (D-P6); Err is for logs/human errors.
type Formatter struct {
	Mode Mode
	Out  io.Writer
	Err  io.Writer
}

type okEnvelope struct {
	OK            bool `json:"ok"`
	SchemaVersion int  `json:"schema_version"`
	Data          any  `json:"data"`
}

type errEnvelope struct {
	OK            bool        `json:"ok"`
	SchemaVersion int         `json:"schema_version"`
	Error         errPayload  `json:"error"`
}

type errPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// EmitOK writes a success envelope/representation.
func (f *Formatter) EmitOK(data any) error {
	switch f.Mode {
	case ModeQuiet:
		return nil
	case ModeJSON:
		enc := json.NewEncoder(f.Out)
		enc.SetEscapeHTML(false)
		return enc.Encode(okEnvelope{OK: true, SchemaVersion: SchemaVersion, Data: data})
	default:
		if hr, ok := data.(HumanRenderer); ok {
			return hr.RenderHuman(f.Out)
		}
		_, err := fmt.Fprintln(f.Out, data)
		return err
	}
}

// EmitError writes an error envelope. Per D-P3, JSON errors go to Out (stdout).
func (f *Formatter) EmitError(code, msg string, details map[string]any) error {
	switch f.Mode {
	case ModeQuiet:
		return nil
	case ModeJSON:
		enc := json.NewEncoder(f.Out)
		enc.SetEscapeHTML(false)
		return enc.Encode(errEnvelope{
			OK:            false,
			SchemaVersion: SchemaVersion,
			Error:         errPayload{Code: code, Message: msg, Details: details},
		})
	default:
		_, err := fmt.Fprintf(f.Err, "error: %s\n", msg)
		return err
	}
}

// ParseMode resolves --json/--quiet flags into a Mode (D-07).
func ParseMode(jsonFlag, quiet bool) (Mode, error) {
	if jsonFlag && quiet {
		return ModeHuman, errors.New("--json and --quiet are mutually exclusive")
	}
	switch {
	case jsonFlag:
		return ModeJSON, nil
	case quiet:
		return ModeQuiet, nil
	default:
		return ModeHuman, nil
	}
}
