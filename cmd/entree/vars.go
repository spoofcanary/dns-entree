package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ParseVarsFlags merges a --vars-file JSON object (loaded first) with repeatable
// --var key=value flags (applied after, so they override the file) per D-10.
// Values are checked for injection characters (newline, CR, NUL) per TMPL-14.
func ParseVarsFlags(varFlags []string, varsFile string) (map[string]string, error) {
	out := map[string]string{}

	if varsFile != "" {
		data, err := os.ReadFile(varsFile)
		if err != nil {
			return nil, &UserError{
				Code: "INVALID_VARS_FILE",
				Msg:  fmt.Sprintf("cannot read vars file %s", varsFile),
			}
		}
		// Require strict string map: decoding into map[string]string will fail
		// if any value is not a JSON string.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, &UserError{
				Code: "INVALID_VARS_FILE",
				Msg:  "vars file is not a JSON object of string values",
			}
		}
		for k, v := range raw {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return nil, &UserError{
					Code: "INVALID_VARS_FILE",
					Msg:  fmt.Sprintf("vars file key %q is not a string", k),
				}
			}
			if err := checkVarValue(s); err != nil {
				return nil, err
			}
			out[k] = s
		}
	}

	for _, kv := range varFlags {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			return nil, &UserError{
				Code: "INVALID_VAR",
				Msg:  fmt.Sprintf("--var %q must be key=value", kv),
			}
		}
		k := kv[:eq]
		v := kv[eq+1:]
		if err := checkVarValue(v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

func checkVarValue(v string) error {
	if strings.ContainsAny(v, "\n\r\x00") {
		return &UserError{
			Code: "INVALID_VAR_VALUE",
			Msg:  "variable values must not contain newline, carriage return, or null byte",
		}
	}
	return nil
}
