package migrate

import entree "github.com/spoofcanary/dns-entree"

// Zone is the result of scraping or importing a DNS zone. The shape is
// identical across the three input modes (AXFR, iterated, BIND) so downstream
// orchestration is mode-agnostic. See decision D-04.
type Zone struct {
	Domain      string
	Records     []entree.Record
	Source      string // "axfr" | "iterated" | "bind"
	Nameservers []string
	Warnings    []string
}
