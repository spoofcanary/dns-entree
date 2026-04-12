package entree

import (
	"strings"
	"testing"
)

func TestValidateDNSName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid names
		{"simple domain", "example.com", false},
		{"subdomain", "sub.example.com", false},
		{"trailing dot", "example.com.", false},
		{"underscore dkim", "_dmarc.example.com", false},
		{"underscore bimi", "_bimi.example.com", false},
		{"wildcard", "*.example.com", false},
		{"punycode", "xn--nxasmq6b.example.com", false},
		{"numeric labels", "123.456.example.com", false},
		{"hyphen middle", "my-domain.example.com", false},
		{"exactly 63 char label", strings.Repeat("a", 63) + ".com", false},
		{"exactly 253 chars total", strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 57) + ".com", false},
		{"deep subdomain", "a.b.c.d.e.f.example.com", false},
		{"mixed case", "Example.COM", false},
		{"underscore in label", "_srv._tcp.example.com", false},

		// Invalid names
		{"empty string", "", true},
		{"consecutive dots", "example..com", true},
		{"bracket", "a]b.com", true},
		{"label too long", strings.Repeat("a", 64) + ".com", true},
		{"total too long", strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63) + ".com", true},
		{"space in name", "exam ple.com", true},
		{"tab in name", "exam\tple.com", true},
		{"newline in name", "exam\nple.com", true},
		{"slash", "example/com", true},
		{"question mark", "example?.com", true},
		{"hash", "example#.com", true},
		{"leading hyphen label", "-example.com", true},
		{"trailing hyphen label", "example-.com", true},
		{"no TLD single label", "localhost", true},
		{"leading dot only", ".com", true},
		{"wildcard not first label", "example.*.com", true},
		{"null byte", "example\x00.com", true},
		{"at sign", "user@example.com", true},
		{"colon", "example:80.com", true},
		{"backslash", "example\\.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDNSName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDNSName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecordValue(t *testing.T) {
	tests := []struct {
		name       string
		recordType string
		value      string
		wantErr    bool
	}{
		// A records
		{"valid A ipv4", "A", "192.168.1.1", false},
		{"valid A public", "A", "8.8.8.8", false},
		{"invalid A not-ip", "A", "not-an-ip", true},
		{"invalid A ipv6 in A", "A", "::1", true},
		{"invalid A empty", "A", "", true},

		// AAAA records
		{"valid AAAA loopback", "AAAA", "::1", false},
		{"valid AAAA full", "AAAA", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},
		{"invalid AAAA ipv4", "AAAA", "192.168.1.1", true},
		{"invalid AAAA empty", "AAAA", "", true},
		{"invalid AAAA garbage", "AAAA", "not-an-ip", true},

		// CNAME records
		{"valid CNAME", "CNAME", "target.example.com", false},
		{"valid CNAME trailing dot", "CNAME", "target.example.com.", false},
		{"invalid CNAME empty", "CNAME", "", true},
		{"invalid CNAME spaces", "CNAME", "bad domain.com", true},

		// MX records
		{"valid MX", "MX", "10 mail.example.com", false},
		{"valid MX zero priority", "MX", "0 mail.example.com", false},
		{"invalid MX no number", "MX", "notanumber mail.example.com", true},
		{"invalid MX no target", "MX", "10", true},
		{"invalid MX empty", "MX", "", true},
		{"invalid MX negative", "MX", "-1 mail.example.com", true},

		// TXT records
		{"valid TXT spf", "TXT", "v=spf1 include:_spf.google.com ~all", false},
		{"valid TXT dmarc", "TXT", "v=DMARC1; p=reject; rua=mailto:d@example.com", false},
		{"invalid TXT empty", "TXT", "", true},
		{"invalid TXT too long", "TXT", strings.Repeat("a", 4097), true},

		// NS records
		{"valid NS", "NS", "ns1.example.com", false},
		{"invalid NS empty", "NS", "", true},

		// SRV records
		{"valid SRV", "SRV", "10 5 5060 sip.example.com", false},
		{"invalid SRV empty", "SRV", "", true},

		// Unknown types pass through
		{"unknown type passes", "UNKNOWN_TYPE", "anything", false},
		{"unknown type empty passes", "UNKNOWN_TYPE", "", false},
		{"CAA passes", "CAA", "0 issue letsencrypt.org", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRecordValue(tt.recordType, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRecordValue(%q, %q) error = %v, wantErr %v", tt.recordType, tt.value, err, tt.wantErr)
			}
		})
	}
}
