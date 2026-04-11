package migrate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const r53ListEmpty = `<?xml version="1.0"?>
<ListHostedZonesByNameResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <HostedZones/>
  <DNSName>example.com</DNSName>
  <IsTruncated>false</IsTruncated>
  <MaxItems>100</MaxItems>
</ListHostedZonesByNameResponse>`

const r53ListExisting = `<?xml version="1.0"?>
<ListHostedZonesByNameResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <HostedZones>
    <HostedZone>
      <Id>/hostedzone/Z123ABC</Id>
      <Name>example.com.</Name>
      <CallerReference>x</CallerReference>
      <Config><PrivateZone>false</PrivateZone></Config>
      <ResourceRecordSetCount>2</ResourceRecordSetCount>
    </HostedZone>
  </HostedZones>
  <DNSName>example.com</DNSName>
  <IsTruncated>false</IsTruncated>
  <MaxItems>100</MaxItems>
</ListHostedZonesByNameResponse>`

const r53GetZone = `<?xml version="1.0"?>
<GetHostedZoneResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <HostedZone>
    <Id>/hostedzone/Z123ABC</Id>
    <Name>example.com.</Name>
    <CallerReference>x</CallerReference>
    <Config><PrivateZone>false</PrivateZone></Config>
    <ResourceRecordSetCount>2</ResourceRecordSetCount>
  </HostedZone>
  <DelegationSet>
    <NameServers>
      <NameServer>ns-1.awsdns-01.com</NameServer>
      <NameServer>ns-2.awsdns-02.net</NameServer>
    </NameServers>
  </DelegationSet>
</GetHostedZoneResponse>`

const r53CreateOK = `<?xml version="1.0"?>
<CreateHostedZoneResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <HostedZone>
    <Id>/hostedzone/ZNEW999</Id>
    <Name>example.com.</Name>
    <CallerReference>entree-migrate-1</CallerReference>
    <Config><PrivateZone>false</PrivateZone></Config>
    <ResourceRecordSetCount>2</ResourceRecordSetCount>
  </HostedZone>
  <ChangeInfo>
    <Id>/change/C1</Id>
    <Status>PENDING</Status>
    <SubmittedAt>2026-04-06T00:00:00Z</SubmittedAt>
  </ChangeInfo>
  <DelegationSet>
    <NameServers>
      <NameServer>ns-3.awsdns-03.org</NameServer>
      <NameServer>ns-4.awsdns-04.co.uk</NameServer>
    </NameServers>
  </DelegationSet>
</CreateHostedZoneResponse>`

func TestRoute53Adapter_ExistingZone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		switch {
		case strings.Contains(r.URL.Path, "hostedzonesbyname"):
			_, _ = w.Write([]byte(r53ListExisting))
		case strings.Contains(r.URL.Path, "/hostedzone/Z123ABC"):
			_, _ = w.Write([]byte(r53GetZone))
		default:
			t.Errorf("unexpected req: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a, _ := GetAdapter("route53")
	info, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{
		AccessKey: "AKIA",
		SecretKey: "secret",
		Region:    "us-east-1",
		Endpoint:  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Created {
		t.Error("expected Created=false")
	}
	if info.ZoneID != "Z123ABC" {
		t.Errorf("zone id = %q", info.ZoneID)
	}
	if len(info.Nameservers) != 2 || info.Nameservers[0] != "ns-1.awsdns-01.com" {
		t.Errorf("ns = %v", info.Nameservers)
	}
}

func TestRoute53Adapter_CreateZone(t *testing.T) {
	var sawCreate bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "hostedzonesbyname"):
			_, _ = w.Write([]byte(r53ListEmpty))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/hostedzone"):
			sawCreate = true
			_, _ = w.Write([]byte(r53CreateOK))
		default:
			t.Errorf("unexpected req: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a, _ := GetAdapter("route53")
	info, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{
		AccessKey: "AKIA",
		SecretKey: "secret",
		Endpoint:  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawCreate {
		t.Error("expected POST CreateHostedZone")
	}
	if !info.Created {
		t.Error("expected Created=true")
	}
	if info.ZoneID != "ZNEW999" {
		t.Errorf("zone id = %q", info.ZoneID)
	}
	if len(info.Nameservers) != 2 || info.Nameservers[1] != "ns-4.awsdns-04.co.uk" {
		t.Errorf("ns = %v", info.Nameservers)
	}
}

func TestRoute53Adapter_RejectsBadInputs(t *testing.T) {
	a, _ := GetAdapter("route53")
	if _, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{}); err == nil {
		t.Error("expected error without credentials")
	}
	if _, err := a.EnsureZone(context.Background(), "", ProviderOpts{AccessKey: "a", SecretKey: "b"}); err == nil {
		t.Error("expected error empty domain")
	}
	if _, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{AccessKey: "a", SecretKey: "b", Endpoint: "ftp://x"}); err == nil {
		t.Error("expected error bad endpoint")
	}
}
