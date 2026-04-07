package main

import "errors"

const (
	ExitOK         = 0
	ExitRuntimeErr = 1
	ExitUserErr    = 2
)

// UserError represents a caller-side problem (bad flags, missing creds, invalid input).
type UserError struct {
	Code    string
	Msg     string
	Details map[string]any
}

func (e *UserError) Error() string { return e.Msg }

// RuntimeError represents a system/runtime failure (network, provider API, DNS).
type RuntimeError struct {
	Code    string
	Msg     string
	Details map[string]any
}

func (e *RuntimeError) Error() string { return e.Msg }

// ClassifyExit maps an error to a stable exit code (D-08).
func ClassifyExit(err error) int {
	if err == nil {
		return ExitOK
	}
	var ue *UserError
	if errors.As(err, &ue) {
		return ExitUserErr
	}
	var re *RuntimeError
	if errors.As(err, &re) {
		return ExitRuntimeErr
	}
	return ExitRuntimeErr
}
