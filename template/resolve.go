package template

import (
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"

	entree "github.com/spoofcanary/dns-entree"
)

// varRegex matches Domain Connect %name% style variables.
var varRegex = regexp.MustCompile(`%([a-zA-Z0-9_]+)%`)

// supportedTypes per D-08. SPFM is recognized so apply.go can route it later.
var supportedTypes = map[string]bool{
	"TXT": true, "CNAME": true, "A": true, "AAAA": true,
	"MX": true, "NS": true, "SRV": true, "SPFM": true,
}

// ResolvedRecord pairs a concrete entree.Record with the conflict-mode info
// from its source TemplateRecord. ApplyTemplate consumes this to drive the
// per-record conflict handling in apply.go (D-17).
type ResolvedRecord struct {
	Record entree.Record
	Mode   string
	Prefix string
}

// ResolveDetailed is Resolve plus per-record conflict metadata.
func (t *Template) ResolveDetailed(vars map[string]string) ([]ResolvedRecord, error) {
	logger := t.logger
	if logger == nil {
		logger = slog.Default()
	}
	out := make([]ResolvedRecord, 0, len(t.Records))
	for i, r := range t.Records {
		typ := strings.ToUpper(strings.TrimSpace(r.Type))
		if !supportedTypes[typ] {
			logger.Warn("template: skipping unknown record type", "index", i, "type", r.Type)
			continue
		}
		recs, err := t.resolveOne(i, r, typ, vars)
		if err != nil {
			return nil, err
		}
		prefix, err := substitute(r.TxtConflictMatchingPrefix, vars, i, "txtConflictMatchingPrefix")
		if err != nil {
			return nil, err
		}
		out = append(out, ResolvedRecord{Record: recs, Mode: r.TxtConflictMatchingMode, Prefix: prefix})
	}
	return out, nil
}

// resolveOne resolves a single TemplateRecord into an entree.Record. Shared by
// Resolve and ResolveDetailed.
func (t *Template) resolveOne(i int, r TemplateRecord, typ string, vars map[string]string) (entree.Record, error) {
	pointsTo := r.PointsTo
	if pointsTo == "" {
		pointsTo = r.Target
	}
	host, err := substitute(r.Host, vars, i, "host")
	if err != nil {
		return entree.Record{}, err
	}
	pointsToSub, err := substitute(pointsTo, vars, i, "pointsTo")
	if err != nil {
		return entree.Record{}, err
	}
	dataSub, err := substitute(r.Data, vars, i, "data")
	if err != nil {
		return entree.Record{}, err
	}
	if err := validateHost(host); err != nil {
		return entree.Record{}, fmt.Errorf("template: record %d host: %w", i, err)
	}
	if typ == "TXT" {
		if err := validateTXTData(dataSub); err != nil {
			return entree.Record{}, fmt.Errorf("template: record %d data: %w", i, err)
		}
	}
	if typ == "CNAME" || typ == "A" || typ == "AAAA" || typ == "MX" || typ == "NS" || typ == "SRV" {
		if err := validatePointsTo(typ, pointsToSub); err != nil {
			return entree.Record{}, fmt.Errorf("template: record %d pointsTo: %w", i, err)
		}
	}
	rec := entree.Record{Type: typ, Name: host, TTL: r.TTL}
	switch typ {
	case "TXT", "SPFM":
		rec.Content = dataSub
	case "MX":
		rec.Content = pointsToSub
		rec.Priority = r.Priority
	case "SRV":
		rec.Content = pointsToSub
		rec.Priority = r.Priority
		rec.Weight = r.Weight
		rec.Port = r.Port
		rec.Service = r.Service
		rec.Protocol = r.Protocol
	default:
		rec.Content = pointsToSub
	}
	return rec, nil
}

// Resolve substitutes variables and validates each TemplateRecord, returning
// a slice of concrete entree.Records. Unknown record types are skipped with
// a warning per D-18.
func (t *Template) Resolve(vars map[string]string) ([]entree.Record, error) {
	logger := t.logger
	if logger == nil {
		logger = slog.Default()
	}

	out := make([]entree.Record, 0, len(t.Records))
	for i, r := range t.Records {
		typ := strings.ToUpper(strings.TrimSpace(r.Type))
		if !supportedTypes[typ] {
			logger.Warn("template: skipping unknown record type",
				"index", i, "type", r.Type)
			continue
		}

		// PointsTo source: prefer pointsTo, fall back to target.
		pointsTo := r.PointsTo
		if pointsTo == "" {
			pointsTo = r.Target
		}

		host, err := substitute(r.Host, vars, i, "host")
		if err != nil {
			return nil, err
		}
		pointsToSub, err := substitute(pointsTo, vars, i, "pointsTo")
		if err != nil {
			return nil, err
		}
		dataSub, err := substitute(r.Data, vars, i, "data")
		if err != nil {
			return nil, err
		}
		prefixSub, err := substitute(r.TxtConflictMatchingPrefix, vars, i, "txtConflictMatchingPrefix")
		if err != nil {
			return nil, err
		}
		_ = prefixSub // not part of entree.Record; consumed in apply.go later

		// Per-field validation, post-substitution (D-10).
		if err := validateHost(host); err != nil {
			return nil, fmt.Errorf("template: record %d host: %w", i, err)
		}
		if typ == "TXT" {
			if err := validateTXTData(dataSub); err != nil {
				return nil, fmt.Errorf("template: record %d data: %w", i, err)
			}
		}
		if typ == "CNAME" || typ == "A" || typ == "AAAA" || typ == "MX" || typ == "NS" || typ == "SRV" {
			if err := validatePointsTo(typ, pointsToSub); err != nil {
				return nil, fmt.Errorf("template: record %d pointsTo: %w", i, err)
			}
		}
		// SPFM passes through; apply.go handles it.

		rec := entree.Record{
			Type: typ,
			Name: host,
			TTL:  r.TTL,
		}
		switch typ {
		case "TXT", "SPFM":
			rec.Content = dataSub
		case "MX":
			rec.Content = pointsToSub
			rec.Priority = r.Priority
		case "SRV":
			rec.Content = pointsToSub
			rec.Priority = r.Priority
			rec.Weight = r.Weight
			rec.Port = r.Port
			rec.Service = r.Service
			rec.Protocol = r.Protocol
		default: // CNAME, A, AAAA, NS
			rec.Content = pointsToSub
		}
		out = append(out, rec)
	}
	return out, nil
}

// substitute applies %var% replacement. Missing variables produce an error.
func substitute(in string, vars map[string]string, recIdx int, field string) (string, error) {
	var missErr error
	out := varRegex.ReplaceAllStringFunc(in, func(match string) string {
		name := match[1 : len(match)-1]
		v, ok := vars[name]
		if !ok {
			if missErr == nil {
				missErr = fmt.Errorf("template: record %d %s: missing variable %q", recIdx, field, name)
			}
			return match
		}
		return v
	})
	if missErr != nil {
		return "", missErr
	}
	return out, nil
}

// hasForbiddenChars rejects DNS-meta and control chars per D-09/D-14.
func hasForbiddenChars(s string) error {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == 0:
			return fmt.Errorf("contains null byte")
		case c == '\r':
			return fmt.Errorf("contains carriage return")
		case c == '\n':
			return fmt.Errorf("contains newline")
		case c < 0x20 && c != '\t':
			return fmt.Errorf("contains control character 0x%02x", c)
		case c == 0x7f:
			return fmt.Errorf("contains DEL character")
		}
	}
	return nil
}

// validateTXTData enforces D-09 TXT rules. Bare ; is allowed (it appears
// legitimately in DMARC/SPF), but newlines/null/controls are not.
func validateTXTData(s string) error {
	return hasForbiddenChars(s)
}

// validateHost enforces DNS label rules per D-09. Empty and "@" are accepted
// as the apex.
func validateHost(host string) error {
	if err := hasForbiddenChars(host); err != nil {
		return err
	}
	if host == "" || host == "@" {
		return nil
	}
	if strings.ContainsAny(host, " \t;") {
		return fmt.Errorf("host contains forbidden character")
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return fmt.Errorf("host has leading/trailing dot")
	}
	if len(host) > 253 {
		return fmt.Errorf("host exceeds 253 characters")
	}
	for _, label := range strings.Split(host, ".") {
		if err := validateDNSLabel(label); err != nil {
			return err
		}
	}
	return nil
}

// validateDNSLabel enforces single-label rules: 1..63 chars, [a-zA-Z0-9-_*],
// no leading/trailing hyphen. Underscore allowed (used in _dmarc, _bimi, SRV).
// Asterisk allowed only as a sole label (wildcard).
func validateDNSLabel(label string) error {
	if len(label) == 0 {
		return fmt.Errorf("empty DNS label")
	}
	if len(label) > 63 {
		return fmt.Errorf("DNS label exceeds 63 characters")
	}
	if label == "*" {
		return nil
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return fmt.Errorf("DNS label has leading or trailing hyphen")
	}
	for i := 0; i < len(label); i++ {
		c := label[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_'
		if !ok {
			return fmt.Errorf("DNS label contains invalid character %q", c)
		}
	}
	return nil
}

// validatePointsTo enforces FQDN/IP rules per D-09 for non-TXT targets.
func validatePointsTo(typ, val string) error {
	if err := hasForbiddenChars(val); err != nil {
		return err
	}
	if val == "" {
		return fmt.Errorf("pointsTo is empty")
	}
	if strings.ContainsAny(val, " \t;") {
		return fmt.Errorf("pointsTo contains forbidden character")
	}
	if strings.Contains(val, "://") {
		return fmt.Errorf("pointsTo must not contain a URL scheme")
	}
	if strings.ContainsAny(val, "/?#") {
		return fmt.Errorf("pointsTo must not contain path or query")
	}

	switch typ {
	case "A":
		ip := net.ParseIP(val)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("invalid IPv4 address")
		}
		return nil
	case "AAAA":
		ip := net.ParseIP(val)
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("invalid IPv6 address")
		}
		return nil
	}

	// FQDN validation: strip trailing dot, ensure each label valid, allow port? No.
	if strings.Contains(val, ":") {
		return fmt.Errorf("pointsTo must not contain a port")
	}
	host := strings.TrimSuffix(val, ".")
	if len(host) == 0 || len(host) > 253 {
		return fmt.Errorf("pointsTo length out of range")
	}
	for _, label := range strings.Split(host, ".") {
		if err := validateDNSLabel(label); err != nil {
			return err
		}
	}
	return nil
}
