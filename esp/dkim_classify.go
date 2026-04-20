package esp

import (
	"context"
	"net"
	"strings"
	"time"
)

// DKIMResolver looks up CNAME records for a host. Implementations typically
// wrap net.Resolver but tests may substitute fakes. Matches the net package
// shape: returns the canonical target name or empty string if the name is
// not a CNAME.
type DKIMResolver interface {
	LookupCNAME(ctx context.Context, host string) (string, error)
}

// netDKIMResolver adapts net.Resolver to DKIMResolver with a per-call timeout.
type netDKIMResolver struct {
	r       *net.Resolver
	timeout time.Duration
}

// NewNetDKIMResolver returns a DKIMResolver backed by the stdlib net.Resolver
// with a per-lookup timeout (2s default if zero).
func NewNetDKIMResolver(timeout time.Duration) DKIMResolver {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &netDKIMResolver{r: net.DefaultResolver, timeout: timeout}
}

func (n *netDKIMResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()
	return n.r.LookupCNAME(ctx, host)
}

// DefaultDKIMSelectors is a practical set of selector names to probe when
// the caller has no prior knowledge of which selectors a domain uses.
// Covers common ESPs (SendGrid s1/s2, Google google, M365 selector1/2,
// SES easy-DKIM tokens start with random hashes and won't match, but
// Resend/Postmark/Mailgun/Mailchimp selectors are included).
var DefaultDKIMSelectors = []string{
	"google", "selector1", "selector2",
	"s1", "s2", // SendGrid
	"mte1", "mte2", // Mandrill
	"k1", "k2", "k3", // Mailchimp
	"resend", "resend2", // Resend
	"pm", "pm-bounces", // Postmark
	"mailo", "smtp", // Mailgun
	"default", "dkim", "email",
	"loops",              // Loops
	"bento",              // Bento
	"cm",                 // Customer.io
	"salesforce", "sfdc", // Salesforce
	"hs1-hubspotemail", "hs2-hubspotemail", // HubSpot
}

// ClassifyFromDKIM probes each selector under domain and attempts to
// classify senders based on CNAME targets. Selectors returning no CNAME
// (TXT-only DKIM records or NXDOMAIN) are skipped.
//
// Pass nil selectors to probe DefaultDKIMSelectors.
func ClassifyFromDKIM(ctx context.Context, resolver DKIMResolver, domain string, selectors []string) []SenderClassification {
	if resolver == nil || domain == "" {
		return nil
	}
	if selectors == nil {
		selectors = DefaultDKIMSelectors
	}
	out := make([]SenderClassification, 0, 4)
	seenNames := make(map[string]bool)

	for _, sel := range selectors {
		host := sel + "._domainkey." + strings.TrimSuffix(domain, ".")
		target, err := resolver.LookupCNAME(ctx, host)
		if err != nil || target == "" {
			continue
		}
		target = strings.TrimSuffix(strings.ToLower(target), ".")
		if target == host || target == strings.TrimSuffix(host, ".") {
			// No real CNAME present (net.Resolver returns the queried name).
			continue
		}

		info, ok := LookupByDKIMTarget(target)
		if !ok {
			continue
		}
		// Dedup by sender name so we don't emit s1 + s2 as two SendGrids.
		if seenNames[info.Name] {
			continue
		}
		seenNames[info.Name] = true

		out = append(out, SenderClassification{
			Name:           info.Name,
			Category:       info.Category,
			Infrastructure: info.Infrastructure,
			Integration:    info.Integration,
			DKIMSelector:   sel,
			DKIMTarget:     target,
		})
	}
	return out
}
