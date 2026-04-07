package main

import "os"

// IsTTY reports whether f refers to a character device (terminal).
func IsTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// RequireYes enforces D-11: under non-TTY, destructive ops must pass --yes explicitly.
func RequireYes(stdoutTTY, yes bool) error {
	if !stdoutTTY && !yes {
		return &UserError{
			Code: "CONFIRM_REQUIRED",
			Msg:  "non-interactive session requires explicit --yes for write operations",
		}
	}
	return nil
}
