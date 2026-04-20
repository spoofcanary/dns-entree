// Package inspect provides a single consolidated lookup pass over a
// domain's DNS and email configuration. See DomainState for the returned
// shape. Exists as a subpackage to avoid an import cycle between the
// root entree package (which exposes SPF helpers) and the esp package
// (which depends on those helpers).
package inspect

import (
	"context"
	"net"
	"strings"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/esp"
)

// DomainState is the consolidated, single-pass snapshot of a domain's email
// and DNS configuration. Inspect returns this so callers don't have to
// stitch together SPF parsing, DKIM resolution, DMARC inspection, NS
// classification, and ESP detection from separate code paths.
//
// Every field is optional from a populated sense — if a lookup fails or
// the record is absent, the field is zero-valued. Callers should treat
// this as an observation, not a guarantee.
type DomainState struct {
	Domain string `json:"domain"`

	// DNS layer
	Nameservers      []string     `json:"nameservers,omitempty"`
	DNSProvider      entree.ProviderType `json:"dns_provider,omitempty"`
	DNSProviderLabel string       `json:"dns_provider_label,omitempty"`
	DNSProviderSupported bool     `json:"dns_provider_supported"` // tier-1 API support
	DNSProviderMethod string      `json:"dns_provider_method,omitempty"` // ns_pattern | rdap_fallback

	// Mail layer
	MXHosts         []string `json:"mx_hosts,omitempty"`
	MailboxProvider string   `json:"mailbox_provider,omitempty"` // friendly name, e.g., "Google Workspace"

	// SPF
	SPFRecord      string   `json:"spf_record,omitempty"`
	SPFIncludes    []string `json:"spf_includes,omitempty"`
	SPFLookupCount int      `json:"spf_lookup_count,omitempty"`

	// DKIM (selectors we probed)
	DKIMSelectors []DKIMSelectorState `json:"dkim_selectors,omitempty"`

	// DMARC
	DMARCRecord string   `json:"dmarc_record,omitempty"`
	DMARCPolicy string   `json:"dmarc_policy,omitempty"` // "none" | "quarantine" | "reject"
	DMARCRuas   []string `json:"dmarc_ruas,omitempty"`
	DMARCRufs   []string `json:"dmarc_rufs,omitempty"`

	// ReportsToCanonical is true when any rua/ruf address has InspectOpts.CanonicalRuaHost
	// as its domain part. Drives the UI's "should we offer to update rua?"
	// question. Covers all cases: no rua at all, rua points at a competitor,
	// rua uses a legacy host, etc.
	ReportsToCanonical bool `json:"reports_to_canonical"`

	// LegacyRuaDetected is true when any rua/ruf uses InspectOpts.LegacyRuaHost
	// (separate from the canonical host). Used to show a more specific
	// "you renamed" message vs a generic "we aren't receiving reports" one.
	LegacyRuaDetected bool `json:"legacy_rua_detected"`

	// Sender classification via esp
	Senders []esp.SenderClassification `json:"senders,omitempty"`
}

// DKIMSelectorState captures whether a probed DKIM selector exists and,
// if it does, what its CNAME target is so callers can classify the signer.
type DKIMSelectorState struct {
	Selector string `json:"selector"`
	Found    bool   `json:"found"`
	Target   string `json:"target,omitempty"` // CNAME target (if CNAME) or empty for TXT-only selectors
	HasTXT   bool   `json:"has_txt,omitempty"`
}

// InspectOpts configures an Inspect call. Zero value is valid and skips
// both the canonical-rua and legacy-rua checks (leaving ReportsToCanonical
// and LegacyRuaDetected false regardless of the record contents). Callers
// that want those flags populated must supply the hostnames.
type InspectOpts struct {
	// LookupTimeout caps each individual DNS lookup. Zero defaults to 2s.
	LookupTimeout time.Duration

	// DKIMSelectors to probe. Zero defaults to esp.DefaultDKIMSelectors.
	DKIMSelectors []string

	// CanonicalRuaHost is the ingest hostname callers want reports
	// delivered to (e.g., "dmarc.sendcanary.com"). When set, Inspect
	// populates DomainState.ReportsToCanonical with true iff any rua/ruf
	// in the DMARC record has this as its domain part. Zero skips the
	// check. dns-entree itself does not care which host this is.
	CanonicalRuaHost string

	// LegacyRuaHost is an optional secondary hostname for a more specific
	// "you renamed" UX, covering cases where a prior version of the
	// product used a different ingest domain. Zero skips the check.
	LegacyRuaHost string
}

// Inspect performs the single consolidated lookup pass for a domain. It
// parallelizes NS / MX / SPF / DMARC fetches and DKIM probes; results are
// combined into a single DomainState. Returns a populated state on
// partial failure (individual record absence is not an error), and only
// errors out when the context is cancelled or the domain is invalid.
func Inspect(ctx context.Context, domain string, opts InspectOpts) (*DomainState, error) {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if domain == "" {
		return nil, errInvalidDomain
	}
	timeout := opts.LookupTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	selectors := opts.DKIMSelectors
	if selectors == nil {
		selectors = esp.DefaultDKIMSelectors
	}

	state := &DomainState{Domain: domain}

	// --- NS + MX + SPF + DMARC in parallel via goroutines ---
	type nsOut struct {
		hosts []string
		err   error
	}
	type mxOut struct {
		hosts []string
		err   error
	}
	type txtOut struct {
		txts []string
		err  error
	}

	nsCh := make(chan nsOut, 1)
	mxCh := make(chan mxOut, 1)
	spfCh := make(chan txtOut, 1)
	dmarcCh := make(chan txtOut, 1)

	go func() {
		c, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		nsRecs, err := net.DefaultResolver.LookupNS(c, domain)
		hosts := make([]string, 0, len(nsRecs))
		for _, n := range nsRecs {
			hosts = append(hosts, strings.TrimSuffix(n.Host, "."))
		}
		nsCh <- nsOut{hosts: hosts, err: err}
	}()
	go func() {
		c, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		mxRecs, err := net.DefaultResolver.LookupMX(c, domain)
		hosts := make([]string, 0, len(mxRecs))
		for _, m := range mxRecs {
			hosts = append(hosts, strings.TrimSuffix(m.Host, "."))
		}
		mxCh <- mxOut{hosts: hosts, err: err}
	}()
	go func() {
		c, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		txts, err := net.DefaultResolver.LookupTXT(c, domain)
		spfCh <- txtOut{txts: txts, err: err}
	}()
	go func() {
		c, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		txts, err := net.DefaultResolver.LookupTXT(c, "_dmarc."+domain)
		dmarcCh <- txtOut{txts: txts, err: err}
	}()

	ns := <-nsCh
	mx := <-mxCh
	spf := <-spfCh
	dm := <-dmarcCh

	// NS classification
	if ns.err == nil && len(ns.hosts) > 0 {
		state.Nameservers = ns.hosts
		det := entree.DetectFromNS(ns.hosts)
		state.DNSProvider = det.Provider
		state.DNSProviderLabel = det.Label
		state.DNSProviderSupported = det.Supported
		state.DNSProviderMethod = det.Method
	}

	// MX + mailbox inference
	if mx.err == nil {
		state.MXHosts = mx.hosts
		state.MailboxProvider = inferMailbox(mx.hosts)
	}

	// SPF
	if spf.err == nil {
		for _, t := range spf.txts {
			low := strings.ToLower(strings.TrimSpace(t))
			if strings.HasPrefix(low, "v=spf1") {
				state.SPFRecord = t
				state.SPFIncludes = extractSPFIncludesPublic(t)
				state.SPFLookupCount = entree.CountSPFLookups(t)
				break
			}
		}
	}

	// DMARC
	if dm.err == nil {
		for _, t := range dm.txts {
			low := strings.ToLower(strings.TrimSpace(t))
			if strings.HasPrefix(low, "v=dmarc1") {
				state.DMARCRecord = t
				state.DMARCPolicy = parseDMARCPolicy(t)
				state.DMARCRuas = parseDMARCAddrs(t, "rua")
				state.DMARCRufs = parseDMARCAddrs(t, "ruf")
				allAddrs := append(append([]string{}, state.DMARCRuas...), state.DMARCRufs...)
				if opts.CanonicalRuaHost != "" {
					state.ReportsToCanonical = ruaContainsHost(allAddrs, opts.CanonicalRuaHost)
				}
				if opts.LegacyRuaHost != "" {
					state.LegacyRuaDetected = ruaContainsHost(allAddrs, opts.LegacyRuaHost)
				}
				break
			}
		}
	}

	// DKIM probe in parallel (bounded concurrency via goroutine per selector)
	if len(selectors) > 0 {
		state.DKIMSelectors = probeDKIM(ctx, timeout, domain, selectors)
	}

	// ESP / sender classification
	spfResolver := entree.NewNetSPFResolver(timeout)
	dkimResolver := esp.NewNetDKIMResolver(timeout)
	state.Senders = esp.ClassifyDomainWithResolvers(ctx, domain, state.SPFRecord, selectors, spfResolver, dkimResolver)

	return state, nil
}

// inferMailbox maps MX hosts to a customer-facing mailbox provider name.
// Covers the providers common in DMARC-affected org domains. Unknown MX
// returns empty string.
func inferMailbox(mxHosts []string) string {
	for _, h := range mxHosts {
		low := strings.ToLower(h)
		switch {
		case strings.Contains(low, "google.com") || strings.Contains(low, "googlemail.com"):
			return "Google Workspace"
		case strings.Contains(low, "outlook.com") || strings.Contains(low, "protection.outlook.com"):
			return "Microsoft 365"
		case strings.Contains(low, "zoho.com"):
			return "Zoho Mail"
		case strings.Contains(low, "messagingengine.com") || strings.Contains(low, "fastmail"):
			return "Fastmail"
		case strings.Contains(low, "icloud.com") || strings.Contains(low, "mail.me.com"):
			return "iCloud Mail"
		case strings.Contains(low, "yahoodns.net") || strings.Contains(low, "yahoo.com"):
			return "Yahoo Mail"
		case strings.Contains(low, "mimecast"):
			return "Mimecast"
		case strings.Contains(low, "barracuda"):
			return "Barracuda"
		}
	}
	return ""
}

// extractSPFIncludesPublic is the public analogue of extractSPFIncludes
// used elsewhere in this package. Kept unexported by the other name to
// avoid breaking callers; this exposes a copy for inspect.
func extractSPFIncludesPublic(spfRaw string) []string {
	out := make([]string, 0, 6)
	for _, tok := range strings.Fields(spfRaw) {
		low := strings.ToLower(tok)
		if strings.HasPrefix(low, "include:") {
			out = append(out, strings.TrimSuffix(strings.TrimPrefix(low, "include:"), "."))
		}
	}
	return out
}

// parseDMARCPolicy pulls the p= tag value from a DMARC record, lowercased.
// Returns empty string when absent.
func parseDMARCPolicy(record string) string {
	for _, tag := range strings.Split(record, ";") {
		kv := strings.SplitN(strings.TrimSpace(tag), "=", 2)
		if len(kv) != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(kv[0]), "p") {
			return strings.ToLower(strings.TrimSpace(kv[1]))
		}
	}
	return ""
}

// parseDMARCAddrs splits rua= / ruf= mailto lists and strips the "mailto:"
// prefix, returning just the addresses.
func parseDMARCAddrs(record, tagName string) []string {
	tagLower := strings.ToLower(tagName) + "="
	for _, tag := range strings.Split(record, ";") {
		t := strings.TrimSpace(tag)
		if !strings.HasPrefix(strings.ToLower(t), tagLower) {
			continue
		}
		value := strings.TrimPrefix(t, t[:len(tagLower)])
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			p = strings.TrimPrefix(p, "mailto:")
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

// ruaContainsHost reports whether any address in the list has the given
// host as its domain part.
func ruaContainsHost(addrs []string, host string) bool {
	low := strings.ToLower(host)
	for _, a := range addrs {
		al := strings.ToLower(a)
		if strings.Contains(al, "@"+low) {
			return true
		}
	}
	return false
}

// probeDKIM issues CNAME + TXT lookups for each selector in parallel and
// returns a DKIMSelectorState per selector. Missing selectors are
// returned with Found=false so callers can reason about absence.
func probeDKIM(ctx context.Context, timeout time.Duration, domain string, selectors []string) []DKIMSelectorState {
	out := make([]DKIMSelectorState, len(selectors))
	done := make(chan struct{}, len(selectors))
	for i, sel := range selectors {
		i, sel := i, sel
		go func() {
			defer func() { done <- struct{}{} }()
			host := sel + "._domainkey." + domain
			cnameCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			target, err := net.DefaultResolver.LookupCNAME(cnameCtx, host)
			out[i] = DKIMSelectorState{Selector: sel}
			if err == nil && target != "" && strings.TrimSuffix(strings.ToLower(target), ".") != strings.TrimSuffix(strings.ToLower(host), ".") {
				out[i].Found = true
				out[i].Target = strings.TrimSuffix(target, ".")
				return
			}
			// No CNAME — try TXT (some selectors publish DKIM as TXT directly).
			txtCtx, txtCancel := context.WithTimeout(ctx, timeout)
			defer txtCancel()
			txts, terr := net.DefaultResolver.LookupTXT(txtCtx, host)
			if terr == nil {
				for _, t := range txts {
					if strings.Contains(t, "v=DKIM1") || strings.Contains(strings.ToLower(t), "k=rsa") {
						out[i].Found = true
						out[i].HasTXT = true
						return
					}
				}
			}
		}()
	}
	for range selectors {
		<-done
	}
	return out
}

// errInvalidDomain is returned by Inspect when the domain is empty or
// otherwise unparseable.
var errInvalidDomain = errInspect("invalid domain")

type errInspect string

func (e errInspect) Error() string { return string(e) }
