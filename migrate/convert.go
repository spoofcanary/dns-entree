package migrate

import (
	"fmt"
	"strings"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/miekg/dns"
)

// rrToRecord converts a miekg/dns.RR to an entree.Record. It returns
// (record, warning, ok). When ok is false the RR should be skipped; warning
// may still be non-empty (e.g. unsupported type, DNSKEY/DS read-only).
// SOA is always dropped without warning.
func rrToRecord(rr dns.RR) (entree.Record, string, bool) {
	if rr == nil {
		return entree.Record{}, "", false
	}
	hdr := rr.Header()
	name := strings.TrimSuffix(hdr.Name, ".")
	ttl := int(hdr.Ttl)

	switch v := rr.(type) {
	case *dns.SOA:
		return entree.Record{}, "", false
	case *dns.DNSKEY:
		return entree.Record{}, fmt.Sprintf("DNSKEY at %s skipped (target provider manages DNSSEC)", name), false
	case *dns.DS:
		return entree.Record{}, fmt.Sprintf("DS at %s skipped (target provider manages DNSSEC)", name), false
	case *dns.A:
		return entree.Record{Type: "A", Name: name, Content: v.A.String(), TTL: ttl}, "", true
	case *dns.AAAA:
		return entree.Record{Type: "AAAA", Name: name, Content: v.AAAA.String(), TTL: ttl}, "", true
	case *dns.CNAME:
		return entree.Record{Type: "CNAME", Name: name, Content: strings.TrimSuffix(v.Target, "."), TTL: ttl}, "", true
	case *dns.TXT:
		return entree.Record{Type: "TXT", Name: name, Content: strings.Join(v.Txt, ""), TTL: ttl}, "", true
	case *dns.MX:
		return entree.Record{Type: "MX", Name: name, Content: strings.TrimSuffix(v.Mx, "."), TTL: ttl, Priority: int(v.Preference)}, "", true
	case *dns.NS:
		return entree.Record{Type: "NS", Name: name, Content: strings.TrimSuffix(v.Ns, "."), TTL: ttl}, "", true
	case *dns.SRV:
		return entree.Record{Type: "SRV", Name: name, Content: strings.TrimSuffix(v.Target, "."), TTL: ttl, Priority: int(v.Priority), Weight: int(v.Weight), Port: int(v.Port)}, "", true
	case *dns.CAA:
		return entree.Record{Type: "CAA", Name: name, Content: fmt.Sprintf("%d %s %q", v.Flag, v.Tag, v.Value), TTL: ttl}, "", true
	case *dns.TLSA:
		return entree.Record{Type: "TLSA", Name: name, Content: fmt.Sprintf("%d %d %d %s", v.Usage, v.Selector, v.MatchingType, v.Certificate), TTL: ttl}, "", true
	case *dns.SSHFP:
		return entree.Record{Type: "SSHFP", Name: name, Content: fmt.Sprintf("%d %d %s", v.Algorithm, v.Type, v.FingerPrint), TTL: ttl}, "", true
	default:
		return entree.Record{}, fmt.Sprintf("unsupported rrtype %s at %s skipped", dns.TypeToString[hdr.Rrtype], name), false
	}
}

// inZone reports whether name is within zone (case-insensitive, dot-aware).
func inZone(name, zone string) bool {
	n := strings.ToLower(strings.TrimSuffix(name, "."))
	z := strings.ToLower(strings.TrimSuffix(zone, "."))
	return n == z || strings.HasSuffix(n, "."+z)
}
