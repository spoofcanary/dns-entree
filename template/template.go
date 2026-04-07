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
	Data                      string `json:"data"`
	TTL                       int    `json:"ttl"`
	GroupID                   string `json:"groupId"`
	Essential                 string `json:"essential"`
	TxtConflictMatchingMode   string `json:"txtConflictMatchingMode"`
	TxtConflictMatchingPrefix string `json:"txtConflictMatchingPrefix"`

	// MX / SRV optional fields.
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	Port     int    `json:"port"`
	Service  string `json:"service"`
	Protocol string `json:"protocol"`
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
