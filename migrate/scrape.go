package migrate

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/miekg/dns"
)

// ScrapeOptions configures ScrapeZone.
type ScrapeOptions struct {
	// Nameservers is the list of authoritative NS host:port addresses to query.
	// If empty, ScrapeZone resolves NS records via the system resolver.
	Nameservers []string
	// ExtraLabels are added to DefaultLabels during iterated walk.
	ExtraLabels []string
	// OnlyLabels, when non-empty, replaces DefaultLabels entirely.
	OnlyLabels []string
	// QueriesPerSecond limits iterated query rate (default 20).
	QueriesPerSecond int
	// SkipAXFR forces iterated mode.
	SkipAXFR bool
	// Timeout per individual query (default 5s).
	Timeout time.Duration
}

func (o *ScrapeOptions) defaults() {
	if o.QueriesPerSecond <= 0 {
		o.QueriesPerSecond = 20
	}
	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Second
	}
}

func (o *ScrapeOptions) labels() []string {
	if len(o.OnlyLabels) > 0 {
		return append([]string{}, o.OnlyLabels...)
	}
	out := append([]string{}, DefaultLabels...)
	out = append(out, o.ExtraLabels...)
	return out
}

// ScrapeZone extracts a zone from authoritative DNS. It first attempts AXFR
// against each nameserver; on refusal it falls back to iterated label queries.
func ScrapeZone(ctx context.Context, domain string, opts ScrapeOptions) (*Zone, error) {
	opts.defaults()
	domain = strings.TrimSuffix(domain, ".")

	nameservers := opts.Nameservers
	if len(nameservers) == 0 {
		nss, err := net.LookupNS(domain)
		if err != nil {
			return nil, fmt.Errorf("lookup NS for %s: %w", domain, err)
		}
		for _, ns := range nss {
			host := strings.TrimSuffix(ns.Host, ".")
			ips, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil || len(ips) == 0 {
				continue
			}
			nameservers = append(nameservers, net.JoinHostPort(ips[0], "53"))
		}
		if len(nameservers) == 0 {
			return nil, fmt.Errorf("no usable nameservers for %s", domain)
		}
	}

	if !opts.SkipAXFR {
		for _, ns := range nameservers {
			z, err := axfrTransfer(ctx, domain, ns, opts.Timeout)
			if err == nil && z != nil {
				z.Nameservers = nameservers
				return z, nil
			}
		}
	}

	z, err := iteratedScrape(ctx, domain, nameservers, opts)
	if err != nil {
		return nil, err
	}
	z.Nameservers = nameservers
	return z, nil
}

// axfrTransfer attempts a zone transfer from a single nameserver.
func axfrTransfer(ctx context.Context, domain, nsAddr string, timeout time.Duration) (*Zone, error) {
	t := &dns.Transfer{DialTimeout: timeout, ReadTimeout: timeout, WriteTimeout: timeout}
	m := new(dns.Msg)
	m.SetAxfr(dns.Fqdn(domain))

	envCh, err := t.In(m, nsAddr)
	if err != nil {
		return nil, err
	}

	z := &Zone{Domain: domain, Source: "axfr"}
	soaCount := 0
	var rrs []dns.RR
	for env := range envCh {
		if env.Error != nil {
			return nil, env.Error
		}
		rrs = append(rrs, env.RR...)
	}
	if len(rrs) == 0 {
		return nil, fmt.Errorf("axfr returned no records")
	}

	for _, rr := range rrs {
		hdr := rr.Header()
		if hdr.Rrtype == dns.TypeSOA {
			soaCount++
			// RFC 5936: SOA appears at start and end. Drop both.
			continue
		}
		// Reject out-of-zone names (cache poison guard, T-05b-01).
		if !inZone(hdr.Name, domain) {
			z.Warnings = append(z.Warnings, fmt.Sprintf("dropped out-of-zone AXFR record %s %s", dns.TypeToString[hdr.Rrtype], strings.TrimSuffix(hdr.Name, ".")))
			continue
		}
		rec, warn, keep := rrToRecord(rr)
		if warn != "" {
			z.Warnings = append(z.Warnings, warn)
		}
		if !keep {
			continue
		}
		z.Records = append(z.Records, rec)
	}
	if soaCount == 0 {
		return nil, fmt.Errorf("axfr response missing SOA")
	}
	return z, nil
}

// iteratedQueryTypes is the set of record types iterated scrape walks per label.
var iteratedQueryTypes = []uint16{
	dns.TypeA, dns.TypeAAAA, dns.TypeCNAME, dns.TypeTXT,
	dns.TypeMX, dns.TypeNS, dns.TypeSRV, dns.TypeCAA,
	dns.TypeTLSA, dns.TypeSSHFP,
}

// iteratedScrape walks DefaultLabels + extras and queries each record type.
// Per-label failures accumulate as warnings; in-zone CNAME targets enqueue
// further queries up to depth 4.
func iteratedScrape(ctx context.Context, domain string, nameservers []string, opts ScrapeOptions) (*Zone, error) {
	z := &Zone{Domain: domain, Source: "iterated"}

	type queueItem struct {
		name  string
		depth int
	}
	seen := map[string]bool{}
	var queue []queueItem
	enqueue := func(name string, depth int) {
		key := strings.ToLower(strings.TrimSuffix(name, "."))
		if seen[key] {
			return
		}
		seen[key] = true
		queue = append(queue, queueItem{name: key, depth: depth})
	}

	for _, lbl := range opts.labels() {
		var fqdn string
		if lbl == "@" {
			fqdn = domain
		} else {
			fqdn = lbl + "." + domain
		}
		enqueue(fqdn, 0)
	}

	interval := time.Second / time.Duration(opts.QueriesPerSecond)
	if interval <= 0 {
		interval = time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	addedNS := map[string]bool{}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		for _, qtype := range iteratedQueryTypes {
			select {
			case <-ctx.Done():
				return z, ctx.Err()
			case <-ticker.C:
			}

			rrs, rcode, err := queryAuthoritative(ctx, nameservers, item.name, qtype, opts.Timeout)
			if err != nil {
				z.Warnings = append(z.Warnings, fmt.Sprintf("query %s %s failed: %v", dns.TypeToString[qtype], item.name, err))
				continue
			}
			if rcode != dns.RcodeSuccess && rcode != dns.RcodeNameError {
				z.Warnings = append(z.Warnings, fmt.Sprintf("query %s %s rcode=%s", dns.TypeToString[qtype], item.name, dns.RcodeToString[rcode]))
				continue
			}

			for _, rr := range rrs {
				hdr := rr.Header()
				if !inZone(hdr.Name, domain) {
					continue
				}
				rec, warn, keep := rrToRecord(rr)
				if warn != "" {
					z.Warnings = append(z.Warnings, warn)
				}
				if !keep {
					continue
				}
				z.Records = append(z.Records, rec)

				// Enqueue in-zone CNAME targets.
				if c, ok := rr.(*dns.CNAME); ok && item.depth < 4 {
					tgt := strings.TrimSuffix(c.Target, ".")
					if inZone(tgt, domain) {
						enqueue(tgt, item.depth+1)
					}
				}
				// Track NS delegation child zones for the post-pass warning.
				if ns, ok := rr.(*dns.NS); ok {
					nm := strings.TrimSuffix(hdr.Name, ".")
					if !strings.EqualFold(nm, domain) && !addedNS[nm] {
						addedNS[nm] = true
						z.Warnings = append(z.Warnings, fmt.Sprintf("child zone delegation at %s -> %s (not followed)", nm, strings.TrimSuffix(ns.Ns, ".")))
					}
				}
			}
		}
	}

	z.Records = dedupRecords(z.Records)
	return z, nil
}

func dedupRecords(in []entree.Record) []entree.Record {
	seen := map[string]bool{}
	out := make([]entree.Record, 0, len(in))
	for _, r := range in {
		k := r.Type + "|" + strings.ToLower(r.Name) + "|" + r.Content + "|" + fmt.Sprint(r.Priority)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}

// queryAuthoritative sends a non-recursive query to the first responding NS.
func queryAuthoritative(ctx context.Context, nameservers []string, name string, qtype uint16, timeout time.Duration) ([]dns.RR, int, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), qtype)
	msg.RecursionDesired = false

	c := &dns.Client{Net: "udp", Timeout: timeout}
	var lastErr error
	for _, ns := range nameservers {
		resp, _, err := c.ExchangeContext(ctx, msg, ns)
		if err != nil {
			lastErr = err
			continue
		}
		if resp == nil {
			lastErr = fmt.Errorf("nil response")
			continue
		}
		return resp.Answer, resp.Rcode, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no nameservers available")
	}
	return nil, 0, lastErr
}
