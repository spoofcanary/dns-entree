package esp

import (
	"context"
	"fmt"
	"testing"
)

type fakeDKIMResolver struct {
	records map[string]string // host -> CNAME target
}

func (f *fakeDKIMResolver) LookupCNAME(_ context.Context, host string) (string, error) {
	if target, ok := f.records[host]; ok {
		return target + ".", nil // mimic net.Resolver trailing dot
	}
	// net.Resolver returns the queried name with trailing dot when no CNAME
	// exists. Tests should treat that as "no CNAME".
	return host + ".", fmt.Errorf("no cname")
}

func TestClassifyFromDKIM_SendGrid(t *testing.T) {
	resolver := &fakeDKIMResolver{
		records: map[string]string{
			"s1._domainkey.example.com": "s1.domainkey.u1797798.wl049.sendgrid.net",
			"s2._domainkey.example.com": "s2.domainkey.u1797798.wl049.sendgrid.net",
		},
	}
	results := ClassifyFromDKIM(context.Background(), resolver, "example.com", []string{"s1", "s2"})
	if len(results) != 1 { // dedup: two SendGrid selectors collapse into one entry
		t.Fatalf("results=%+v want 1", results)
	}
	r := results[0]
	if r.Name != "SendGrid" {
		t.Errorf("name=%q", r.Name)
	}
	if r.Infrastructure != InfraTwilio {
		t.Errorf("infra=%q", r.Infrastructure)
	}
	if r.DKIMSelector != "s1" {
		t.Errorf("selector=%q want s1", r.DKIMSelector)
	}
}

func TestClassifyFromDKIM_SESTokens(t *testing.T) {
	// SES Easy DKIM uses hashed token selectors like abc123... but when the
	// CNAME target is *.dkim.amazonses.com we should classify as SES.
	resolver := &fakeDKIMResolver{
		records: map[string]string{
			"default._domainkey.example.com": "abc123xyz.dkim.amazonses.com",
		},
	}
	results := ClassifyFromDKIM(context.Background(), resolver, "example.com", []string{"default"})
	if len(results) != 1 {
		t.Fatalf("results=%+v", results)
	}
	if results[0].Infrastructure != InfraSES {
		t.Errorf("infra=%q want %q", results[0].Infrastructure, InfraSES)
	}
}

func TestClassifyFromDKIM_UnknownTargetsSkipped(t *testing.T) {
	resolver := &fakeDKIMResolver{
		records: map[string]string{
			"k1._domainkey.example.com": "k1.some-unknown-host.example",
		},
	}
	results := ClassifyFromDKIM(context.Background(), resolver, "example.com", []string{"k1"})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %+v", results)
	}
}

func TestClassifyFromDKIM_NilResolverReturnsNil(t *testing.T) {
	results := ClassifyFromDKIM(context.Background(), nil, "example.com", nil)
	if results != nil {
		t.Errorf("expected nil, got %+v", results)
	}
}
