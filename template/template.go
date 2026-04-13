package template

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// Template mirrors the Domain Connect template JSON schema.
type Template struct {
	ProviderID          string           `json:"providerId"`
	ProviderName        string           `json:"providerName"`
	ServiceID           string           `json:"serviceId"`
	ServiceName         string           `json:"serviceName"`
	Version             int              `json:"version"`
	LogoURL             string           `json:"logoUrl"`
	Description         string           `json:"description"`
	VariableDescription string           `json:"variableDescription"`
	SyncPubKeyDomain    string           `json:"syncPubKeyDomain"`
	SyncRedirectDomain  string           `json:"syncRedirectDomain"`
	MultiInstance       bool             `json:"multiInstance"`
	HostRequired        bool             `json:"hostRequired"`
	Records             []TemplateRecord `json:"records"`

	logger *slog.Logger
}

// TemplateRecord mirrors a single record entry inside a DC template.
type TemplateRecord struct {
	Type                      string  `json:"type"`
	Host                      string  `json:"host"`
	PointsTo                  string  `json:"pointsTo"`
	Target                    string  `json:"target"` // some templates use "target"
	Data                      string  `json:"data"`
	TTL                       flexInt `json:"ttl"`
	GroupID                   string  `json:"groupId"`
	Essential                 string  `json:"essential"`
	TxtConflictMatchingMode   string  `json:"txtConflictMatchingMode"`
	TxtConflictMatchingPrefix string  `json:"txtConflictMatchingPrefix"`

	// MX / SRV optional fields. flexInt accepts both JSON numbers and quoted
	// strings; some official templates encode "priority": "10".
	Priority flexInt `json:"priority"`
	Weight   flexInt `json:"weight"`
	Port     flexInt `json:"port"`
	Service  string  `json:"service"`
	Protocol string  `json:"protocol"`
}

// flexInt is an int that unmarshals from either a JSON number or a JSON string
// containing an integer. Empty string and JSON null decode to 0. If the string
// contains a Domain Connect %var% token, parsing is deferred: Value is 0 and
// Raw holds the unresolved string for substitution at Resolve time.
type flexInt struct {
	Value int
	Raw   string // set only if the field contains a %var% token
}

// UnmarshalJSON implements lenient int parsing for template fields.
func (f *flexInt) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = flexInt{}
		return nil
	}
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		*f = flexInt{}
		return nil
	}
	if strings.Contains(s, "%") {
		f.Raw = s
		f.Value = 0
		return nil
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return fmt.Errorf("template: invalid integer %q: %w", s, err)
	}
	f.Value = n
	f.Raw = ""
	return nil
}

// singleVarRe matches exactly one bare %variable% with nothing else.
var singleVarRe = regexp.MustCompile(`^%[a-zA-Z0-9_-]+%$`)

// resolve returns the int value, substituting %var% from vars if Raw is set.
// DC spec: integer fields must be either a plain integer literal or exactly
// one bare %variable% token - no concatenation (prefix, suffix, or multiple
// variables).
func (f flexInt) resolve(vars map[string]string, recIdx int, field string) (int, error) {
	if f.Raw == "" {
		return f.Value, nil
	}
	// Strict validation: raw value containing % must be exactly one %var%.
	if strings.Contains(f.Raw, "%") && !singleVarRe.MatchString(f.Raw) {
		return 0, &InvalidDataError{Msg: fmt.Sprintf("record %d %s: integer field %q must be a plain integer or a single %%variable%%", recIdx, field, f.Raw)}
	}
	sub, err := substitute(f.Raw, vars, recIdx, field)
	if err != nil {
		return 0, err
	}
	if sub == "" {
		return 0, nil
	}
	var n int
	if _, err := fmt.Sscanf(sub, "%d", &n); err != nil {
		return 0, fmt.Errorf("template: record %d %s: invalid integer %q after substitution", recIdx, field, sub)
	}
	return n, nil
}

// LoadOption configures Load* calls.
type LoadOption func(*loadConfig)

type loadConfig struct {
	logger *slog.Logger
}

// WithLogger captures a slog.Logger for warnings (e.g. unknown record types).
func WithLogger(l *slog.Logger) LoadOption {
	return func(c *loadConfig) { c.logger = l }
}

// LoadTemplateJSON parses a Domain Connect template from raw JSON bytes.
// Lenient: unknown JSON fields are silently dropped (D-05).
func LoadTemplateJSON(data []byte, opts ...LoadOption) (*Template, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("template: empty JSON input")
	}
	cfg := loadConfig{logger: slog.Default()}
	for _, o := range opts {
		o(&cfg)
	}
	var t Template
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("template: parse JSON: %w", err)
	}
	t.logger = cfg.logger
	return &t, nil
}

// LoadTemplateFile reads a template JSON file from disk and parses it.
func LoadTemplateFile(path string, opts ...LoadOption) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("template: read %s: %w", path, err)
	}
	return LoadTemplateJSON(data, opts...)
}
