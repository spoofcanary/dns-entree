package migrate

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/miekg/dns"
)

// ImportBINDFile parses a BIND-format zone file from path. $INCLUDE directives
// are disabled to prevent arbitrary local file reads (T-05b-02).
func ImportBINDFile(path, domain string) (*Zone, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open bind file: %w", err)
	}
	defer f.Close()
	return ImportBIND(f, domain)
}

// ImportBIND parses a BIND-format zone from r. $INCLUDE is rejected.
func ImportBIND(r io.Reader, domain string) (*Zone, error) {
	z := &Zone{Domain: strings.TrimSuffix(domain, "."), Source: "bind"}

	zp := dns.NewZoneParser(r, dns.Fqdn(domain), "")
	zp.SetIncludeAllowed(false)

	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		rec, warn, keep := rrToRecord(rr)
		if warn != "" {
			z.Warnings = append(z.Warnings, warn)
		}
		if !keep {
			continue
		}
		if !inZone(rec.Name, z.Domain) {
			z.Warnings = append(z.Warnings, fmt.Sprintf("dropped out-of-zone record %s %s", rec.Type, rec.Name))
			continue
		}
		z.Records = append(z.Records, rec)
	}
	if err := zp.Err(); err != nil {
		// $INCLUDE produces a parse error here because SetIncludeAllowed(false).
		return nil, fmt.Errorf("bind parse: %w", err)
	}
	return z, nil
}
