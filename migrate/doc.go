// Package migrate scrapes DNS zones from authoritative nameservers (AXFR or
// iterated label queries) and BIND zone files, producing a uniform Zone struct
// suitable for re-applying through any dns-entree Provider.
//
// See decision D-01 in .planning/phases/05b-migration/05b-CONTEXT.md.
package migrate
