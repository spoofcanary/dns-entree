package migrate

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	entree "github.com/spoofcanary/dns-entree"
)

// nsResolver turns a nameserver hostname (possibly already host:port) into a
// dial target. Overridable in tests to inject an in-process dns.Server.
var nsResolver = func(ctx context.Context, host string) (string, error) {
	h := strings.TrimSuffix(host, ".")
	if _, _, err := net.SplitHostPort(h); err == nil {
		return h, nil
	}
	ips, err := net.DefaultResolver.LookupHost(ctx, h)
	if err != nil || len(ips) == 0 {
		if err == nil {
			err = fmt.Errorf("no IPs for %s", h)
		}
		return "", err
	}
	return net.JoinHostPort(ips[0], "53"), nil
}

// verifyRecordAgainstNS queries the assigned target nameservers directly for
// the given record and returns (matched, gotAny). Matched means the expected
// content appears in the response. gotAny is true if at least one NS responded
// with data for the name (used to distinguish not_yet_propagated from
// mismatch).
func verifyRecordAgainstNS(ctx context.Context, nsHosts []string, rec entree.Record, timeout time.Duration) (matched bool, gotAny bool, detail string) {
	qtype, ok := typeStringToCode(rec.Type)
	if !ok {
		return false, true, "unverifiable type " + rec.Type
	}

	client := &dns.Client{Net: "udp", Timeout: timeout}
	for _, ns := range nsHosts {
		addr, err := nsResolver(ctx, ns)
		if err != nil {
			detail = "resolve NS: " + err.Error()
			continue
		}
		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn(rec.Name), qtype)
		msg.RecursionDesired = false

		resp, _, err := client.ExchangeContext(ctx, msg, addr)
		if err != nil || resp == nil {
			if err != nil {
				detail = err.Error()
			}
			continue
		}
		if len(resp.Answer) > 0 {
			gotAny = true
		}
		for _, rr := range resp.Answer {
			got, _, keep := rrToRecord(rr)
			if !keep {
				continue
			}
			if recordsMatch(rec, got) {
				return true, true, "matched on " + ns
			}
			detail = fmt.Sprintf("got %s=%q want %q", got.Type, got.Content, rec.Content)
		}
	}
	return false, gotAny, detail
}

func recordsMatch(want, got entree.Record) bool {
	if !strings.EqualFold(want.Type, got.Type) {
		return false
	}
	if !strings.EqualFold(strings.TrimSuffix(want.Name, "."), strings.TrimSuffix(got.Name, ".")) {
		return false
	}
	if want.Content != got.Content && !strings.EqualFold(want.Content, got.Content) {
		return false
	}
	if want.Type == "MX" && want.Priority != got.Priority {
		return false
	}
	return true
}

func typeStringToCode(t string) (uint16, bool) {
	switch strings.ToUpper(t) {
	case "A":
		return dns.TypeA, true
	case "AAAA":
		return dns.TypeAAAA, true
	case "CNAME":
		return dns.TypeCNAME, true
	case "TXT":
		return dns.TypeTXT, true
	case "MX":
		return dns.TypeMX, true
	case "NS":
		return dns.TypeNS, true
	case "SRV":
		return dns.TypeSRV, true
	case "CAA":
		return dns.TypeCAA, true
	}
	return 0, false
}

// VerifyResult is the per-record outcome from a single verification round
// against a set of nameservers.
type VerifyResult struct {
	Record entree.Record
	Status string // "matched", "not_yet_propagated", "mismatch", "error"
	Detail string
}

// VerifyTargetSummary is the aggregate result of one verification round.
type VerifyTargetSummary struct {
	Matched int
	Total   int
	Results []VerifyResult
}

// VerifyAgainstNS runs ONE verification round: for each record in records, query
// each nameserver in nameservers directly (no recursion) and return the
// per-record status. This is intended for callers that want to drive their own
// polling loop (e.g., a stateless HTTP poll endpoint), as opposed to Migrate's
// internal verification loop which polls until matched or timeout.
//
// Status values:
//   - "matched": at least one NS returned the expected record content
//   - "not_yet_propagated": NS returned no answer for the name (DNS propagation lag)
//   - "mismatch": NS returned a different value for the name
//   - "error": all NS lookups failed (network, NXDOMAIN, etc)
func VerifyAgainstNS(ctx context.Context, nameservers []string, records []entree.Record, timeout time.Duration) VerifyTargetSummary {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	out := VerifyTargetSummary{Total: len(records), Results: make([]VerifyResult, 0, len(records))}
	for _, rec := range records {
		matched, gotAny, detail := verifyRecordAgainstNS(ctx, nameservers, rec, timeout)
		status := "error"
		switch {
		case matched:
			status = "matched"
			out.Matched++
		case gotAny:
			status = "mismatch"
		case detail != "":
			status = "not_yet_propagated"
		}
		out.Results = append(out.Results, VerifyResult{Record: rec, Status: status, Detail: detail})
	}
	return out
}
