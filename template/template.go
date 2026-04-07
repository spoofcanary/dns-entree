// Package template implements a Domain Connect template parser, resolver,
// and (in later plans) syncer + applier. This file covers the type
// definitions and JSON loading per D-03/D-04/D-05.
package template

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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
	Records             []TemplateRecord `json:"records"`

	logger *slog.Logger
}

// TemplateRecord mirrors a single record entry inside a DC template.
type TemplateRecord struct {
	Type                      string `json:"type"`
	Host                      string `json:"host"`
	PointsTo                  string `json:"pointsTo"`
	Target                    string `json:"target"` // some templates use "target"
	Data                      string  `json:"data"`
	TTL                       flexInt `json:"ttl"`
	GroupID                   string `json:"groupId"`
	Essential                 string `json:"essential"`
	TxtConflictMatchingMode   string `json:"txtConflictMatchingMode"`
	TxtConflictMatchingPrefix string `json:"txtConflictMatchingPrefix"`

	// MX / SRV optional fields. flexInt accepts both JSON numbers and quoted
	// strings; some official templates encode "priority": "10".
	Priority flexInt `json:"priority"`
	Weight   flexInt `json:"weight"`
	Port     flexInt `json:"port"`
	Service  string  `json:"service"`
	Protocol string  `json:"protocol"`
}

// flexInt is an int that unmarshals from either a JSON number or a JSON string
// containing an integer. Empty string and JSON null decode to 0.
type flexInt int

// UnmarshalJSON implements lenient int parsing for template fields.
func (f *flexInt) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = 0
		return nil
	}
	// Strip surrounding quotes if present.
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		*f = 0
		return nil
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return fmt.Errorf("template: invalid integer %q: %w", s, err)
	}
	*f = flexInt(n)
	return nil
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
