package entree

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// VerifyOpts configures a DNS verification query.
type VerifyOpts struct {
	RecordType string // "TXT", "CNAME", "MX", "A", "AAAA"
	Name       string // FQDN of the record, e.g. "_dmarc.example.com"
	Contains   string // optional, case-insensitive substring; empty = any
}

// VerifyResult is the outcome of a Verify call.
type VerifyResult struct {
	Verified           bool
	CurrentValue       string
	Method             string // "authoritative" | "recursive_fallback"
	NameserversQueried []string
}

// Test seams.
var (
	lookupNS = net.LookupNS
	// nsAddrFunc resolves an NS hostname to a "host:port" address used for the UDP query.
	nsAddrFunc = func(ctx context.Context, host string) (string, error) {
		ips, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil || len(ips) == 0 {
			if err == nil {
				err = fmt.Errorf("no IPs for %s", host)
			}
			return "", err
		}
		return net.JoinHostPort(ips[0], "53"), nil
	}
	// recursiveAddr is the address used for recursive fallback queries.
	// Tests override this to point at an in-process server.
	recursiveAddr = ""
	// resolverConfigPath is the path to resolv.conf for discovering the system resolver.
	resolverConfigPath = "/etc/resolv.conf"
)

func recordTypeCode(t string) (uint16, error) {
	switch strings.ToUpper(t) {
	case "TXT":
		return dns.TypeTXT, nil
	case "CNAME":
		return dns.TypeCNAME, nil
	case "MX":
		return dns.TypeMX, nil
	case "A":
		return dns.TypeA, nil
	case "AAAA":
		return dns.TypeAAAA, nil
	default:
		return 0, fmt.Errorf("unsupported record type: %q", t)
	}
}

func extractValues(answers []dns.RR, qtype uint16) []string {
	var out []string
	for _, rr := range answers {
		if rr == nil || rr.Header().Rrtype != qtype {
			continue
		}
		switch v := rr.(type) {
		case *dns.TXT:
			out = append(out, strings.Join(v.Txt, ""))
		case *dns.CNAME:
			out = append(out, strings.TrimSuffix(v.Target, "."))
		case *dns.MX:
			out = append(out, fmt.Sprintf("%d %s", v.Preference, strings.TrimSuffix(v.Mx, ".")))
		case *dns.A:
			out = append(out, v.A.String())
		case *dns.AAAA:
			out = append(out, v.AAAA.String())
		}
	}
	return out
}

func matches(value, contains string) bool {
	if contains == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(contains))
}

func queryServer(ctx context.Context, addr string, name string, qtype uint16, recursion bool) ([]string, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), qtype)
	msg.RecursionDesired = recursion

	client := &dns.Client{Net: "udp", Timeout: 5 * time.Second}
	resp, _, err := client.ExchangeContext(ctx, msg, addr)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return extractValues(resp.Answer, qtype), nil
}

// Verify queries authoritative nameservers for the requested record, falling
// back to recursive resolution. The returned VerifyResult is always usable
// even when Verified is false.
func Verify(ctx context.Context, domain string, opts VerifyOpts) (VerifyResult, error) {
	result := VerifyResult{Method: "authoritative"}

	qtype, err := recordTypeCode(opts.RecordType)
	if err != nil {
		return result, err
	}

	nsRecords, nsErr := lookupNS(domain)

	tried := 0
	if nsErr == nil {
		for _, ns := range nsRecords {
			if tried >= 2 {
				break
			}
			tried++
			nsHost := strings.TrimSuffix(ns.Host, ".")
			result.NameserversQueried = append(result.NameserversQueried, nsHost)

			addr, err := nsAddrFunc(ctx, nsHost)
			if err != nil {
				continue
			}

			values, err := queryServer(ctx, addr, opts.Name, qtype, false)
			if err != nil {
				continue
			}
			for _, v := range values {
				if matches(v, opts.Contains) {
					result.Verified = true
					result.CurrentValue = v
					return result, nil
				}
				if result.CurrentValue == "" {
					result.CurrentValue = v
				}
			}
		}
	}

	// Recursive fallback.
	addr := recursiveAddr
	if addr == "" {
		if cfg, cfgErr := dns.ClientConfigFromFile(resolverConfigPath); cfgErr == nil && len(cfg.Servers) > 0 {
			addr = net.JoinHostPort(cfg.Servers[0], cfg.Port)
		} else {
			addr = "8.8.8.8:53"
		}
	}

	values, recErr := queryServer(ctx, addr, opts.Name, qtype, true)
	if recErr != nil {
		if nsErr != nil {
			return result, fmt.Errorf("authoritative lookup failed (%v) and recursive failed (%w)", nsErr, recErr)
		}
		return result, nil
	}

	for _, v := range values {
		if matches(v, opts.Contains) {
			result.Verified = true
			result.CurrentValue = v
			result.Method = "recursive_fallback"
			return result, nil
		}
		if result.CurrentValue == "" {
			result.CurrentValue = v
			result.Method = "recursive_fallback"
		}
	}
	if len(values) > 0 {
		result.Method = "recursive_fallback"
	}
	return result, nil
}
