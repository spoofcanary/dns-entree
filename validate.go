package entree

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ValidateDNSName validates a DNS name against RFC 1035 with common edge case
// tolerance: trailing dots (fully qualified names), underscores (DKIM, SRV,
// BIMI records), and wildcards as the first label. Returns nil if valid.
func ValidateDNSName(name string) error {
	if name == "" {
		return fmt.Errorf("DNS name is empty")
	}

	// Reject control characters, whitespace, and clearly invalid chars early.
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c <= 0x20 || c == 0x7f {
			return fmt.Errorf("DNS name contains invalid character 0x%02x", c)
		}
	}

	// Strip trailing dot (RFC allows FQDN with trailing dot).
	clean := strings.TrimSuffix(name, ".")

	if clean == "" {
		return fmt.Errorf("DNS name is empty after stripping trailing dot")
	}

	// Total length check (max 253 after stripping trailing dot).
	if len(clean) > 253 {
		return fmt.Errorf("DNS name exceeds 253 characters")
	}

	labels := strings.Split(clean, ".")

	// Must have at least 2 labels (i.e., has a TLD).
	if len(labels) < 2 {
		return fmt.Errorf("DNS name must have at least two labels (missing TLD)")
	}

	for i, label := range labels {
		if err := validateDNSNameLabel(label, i == 0); err != nil {
			return fmt.Errorf("DNS name label %q: %w", label, err)
		}
	}

	return nil
}

// validateDNSNameLabel validates a single DNS label. isFirst indicates whether
// this is the first (leftmost) label, which allows wildcards.
func validateDNSNameLabel(label string, isFirst bool) error {
	if len(label) == 0 {
		return fmt.Errorf("empty label")
	}
	if len(label) > 63 {
		return fmt.Errorf("label exceeds 63 characters")
	}

	// Wildcard: only allowed as sole first label.
	if label == "*" {
		if isFirst {
			return nil
		}
		return fmt.Errorf("wildcard only allowed as first label")
	}

	// Check for leading/trailing hyphens.
	if label[0] == '-' || label[len(label)-1] == '-' {
		return fmt.Errorf("label has leading or trailing hyphen")
	}

	// Allowed characters: a-z, A-Z, 0-9, hyphen, underscore.
	for i := 0; i < len(label); i++ {
		c := label[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_'
		if !ok {
			return fmt.Errorf("label contains invalid character %q", string(rune(c)))
		}
	}

	return nil
}

// ValidateRecordValue validates a DNS record value based on the record type.
// Unknown record types pass through (returns nil) per lenient parsing policy.
func ValidateRecordValue(recordType, value string) error {
	switch strings.ToUpper(recordType) {
	case "A":
		if value == "" {
			return fmt.Errorf("A record value is empty")
		}
		ip := net.ParseIP(value)
		if ip == nil {
			return fmt.Errorf("A record value %q is not a valid IP address", value)
		}
		if ip.To4() == nil {
			return fmt.Errorf("A record value %q is not an IPv4 address", value)
		}
		return nil

	case "AAAA":
		if value == "" {
			return fmt.Errorf("AAAA record value is empty")
		}
		ip := net.ParseIP(value)
		if ip == nil {
			return fmt.Errorf("AAAA record value %q is not a valid IP address", value)
		}
		if ip.To4() != nil {
			return fmt.Errorf("AAAA record value %q is an IPv4 address, not IPv6", value)
		}
		return nil

	case "CNAME":
		if value == "" {
			return fmt.Errorf("CNAME record value is empty")
		}
		return ValidateDNSName(value)

	case "MX":
		if value == "" {
			return fmt.Errorf("MX record value is empty")
		}
		parts := strings.SplitN(value, " ", 2)
		if len(parts) < 2 {
			return fmt.Errorf("MX record value %q missing priority or target", value)
		}
		priority, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("MX record priority %q is not a valid integer", parts[0])
		}
		if priority < 0 {
			return fmt.Errorf("MX record priority must be non-negative, got %d", priority)
		}
		target := strings.TrimSpace(parts[1])
		if target == "" {
			return fmt.Errorf("MX record target is empty")
		}
		return nil

	case "TXT":
		if value == "" {
			return fmt.Errorf("TXT record value is empty")
		}
		if len(value) > 4096 {
			return fmt.Errorf("TXT record value exceeds 4096 characters")
		}
		return nil

	case "NS":
		if value == "" {
			return fmt.Errorf("NS record value is empty")
		}
		return ValidateDNSName(value)

	case "SRV":
		if value == "" {
			return fmt.Errorf("SRV record value is empty")
		}
		return nil

	default:
		// Unknown record types pass through.
		return nil
	}
}
