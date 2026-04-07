package route53

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsr53 "github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	entree "github.com/spoofcanary/dns-entree"
)

type mockR53 struct {
	hostedZones        []r53types.HostedZone
	resourceRecordSets []r53types.ResourceRecordSet

	changeCalled bool
	lastChange   *awsr53.ChangeResourceRecordSetsInput
}

func (m *mockR53) ListHostedZones(ctx context.Context, in *awsr53.ListHostedZonesInput, opts ...func(*awsr53.Options)) (*awsr53.ListHostedZonesOutput, error) {
	return &awsr53.ListHostedZonesOutput{HostedZones: m.hostedZones}, nil
}
func (m *mockR53) ListHostedZonesByName(ctx context.Context, in *awsr53.ListHostedZonesByNameInput, opts ...func(*awsr53.Options)) (*awsr53.ListHostedZonesByNameOutput, error) {
	return &awsr53.ListHostedZonesByNameOutput{HostedZones: m.hostedZones}, nil
}
func (m *mockR53) ListResourceRecordSets(ctx context.Context, in *awsr53.ListResourceRecordSetsInput, opts ...func(*awsr53.Options)) (*awsr53.ListResourceRecordSetsOutput, error) {
	return &awsr53.ListResourceRecordSetsOutput{ResourceRecordSets: m.resourceRecordSets}, nil
}
func (m *mockR53) ChangeResourceRecordSets(ctx context.Context, in *awsr53.ChangeResourceRecordSetsInput, opts ...func(*awsr53.Options)) (*awsr53.ChangeResourceRecordSetsOutput, error) {
	m.changeCalled = true
	m.lastChange = in
	return &awsr53.ChangeResourceRecordSetsOutput{}, nil
}

func zone(name string) r53types.HostedZone {
	return r53types.HostedZone{Id: aws.String("/hostedzone/Z" + name), Name: aws.String(name + ".")}
}

func TestNewProvider_MissingCreds(t *testing.T) {
	_, err := NewProvider("", "secret", "")
	if err == nil || !strings.Contains(err.Error(), "AccessKey required") {
		t.Fatalf("expected AccessKey required, got %v", err)
	}
	_, err = NewProvider("ak", "", "")
	if err == nil || !strings.Contains(err.Error(), "SecretKey required") {
		t.Fatalf("expected SecretKey required, got %v", err)
	}
}

func TestNewProvider_DefaultRegion(t *testing.T) {
	// Just exercise the constructor; the SDK's LoadDefaultConfig is fine offline.
	p, err := NewProvider("ak", "sk", "")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("nil provider")
	}
}

func TestName(t *testing.T) {
	if newProviderWithClient(&mockR53{}).Name() != "Route 53" {
		t.Fail()
	}
}
func TestSlug(t *testing.T) {
	if newProviderWithClient(&mockR53{}).Slug() != "route53" {
		t.Fail()
	}
}

func TestSetRecord_TXTQuoting(t *testing.T) {
	m := &mockR53{hostedZones: []r53types.HostedZone{zone("example.com")}}
	p := newProviderWithClient(m)
	err := p.SetRecord(context.Background(), "example.com", entree.Record{Type: "TXT", Name: "_dmarc.example.com", Content: "v=spf1"})
	if err != nil {
		t.Fatal(err)
	}
	got := aws.ToString(m.lastChange.ChangeBatch.Changes[0].ResourceRecordSet.ResourceRecords[0].Value)
	if got != `"v=spf1"` {
		t.Errorf("got %q, want quoted", got)
	}
}

func TestSetRecord_NonTXTNoQuoting(t *testing.T) {
	m := &mockR53{hostedZones: []r53types.HostedZone{zone("example.com")}}
	p := newProviderWithClient(m)
	err := p.SetRecord(context.Background(), "example.com", entree.Record{Type: "A", Name: "example.com", Content: "1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}
	got := aws.ToString(m.lastChange.ChangeBatch.Changes[0].ResourceRecordSet.ResourceRecords[0].Value)
	if got != "1.2.3.4" {
		t.Errorf("got %q", got)
	}
}

func TestGetRecords_TXTUnquotingAndDot(t *testing.T) {
	m := &mockR53{
		hostedZones: []r53types.HostedZone{zone("example.com")},
		resourceRecordSets: []r53types.ResourceRecordSet{{
			Name: aws.String("_dmarc.example.com."),
			Type: r53types.RRTypeTxt,
			TTL:  aws.Int64(300),
			ResourceRecords: []r53types.ResourceRecord{
				{Value: aws.String(`"v=DMARC1; p=none"`)},
			},
		}},
	}
	p := newProviderWithClient(m)
	recs, err := p.GetRecords(context.Background(), "example.com", "TXT")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records", len(recs))
	}
	if recs[0].Content != "v=DMARC1; p=none" {
		t.Errorf("Content = %q", recs[0].Content)
	}
	if recs[0].Name != "_dmarc.example.com" {
		t.Errorf("Name = %q (trailing dot should be stripped)", recs[0].Name)
	}
}

func TestFindZoneID_LongestSuffix(t *testing.T) {
	m := &mockR53{hostedZones: []r53types.HostedZone{
		zone("example.com"),
		zone("sub.example.com"),
	}}
	p := newProviderWithClient(m)
	id, err := p.findZoneID(context.Background(), "sub.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if id != "Zsub.example.com" {
		t.Errorf("got %q, want longest match Zsub.example.com", id)
	}
}

func TestDeleteRecord_InvalidID(t *testing.T) {
	m := &mockR53{hostedZones: []r53types.HostedZone{zone("example.com")}}
	p := newProviderWithClient(m)
	err := p.DeleteRecord(context.Background(), "example.com", "no-pipe")
	if err == nil || !strings.Contains(err.Error(), "invalid record ID") {
		t.Fatalf("expected invalid record ID, got %v", err)
	}
}

func TestDeleteRecord_IDParsing(t *testing.T) {
	m := &mockR53{
		hostedZones: []r53types.HostedZone{zone("example.com")},
		resourceRecordSets: []r53types.ResourceRecordSet{{
			Name: aws.String("_dmarc.example.com."),
			Type: r53types.RRTypeTxt,
			TTL:  aws.Int64(300),
			ResourceRecords: []r53types.ResourceRecord{
				{Value: aws.String(`"v=DMARC1"`)},
			},
		}},
	}
	p := newProviderWithClient(m)
	err := p.DeleteRecord(context.Background(), "example.com", "_dmarc.example.com|TXT")
	if err != nil {
		t.Fatal(err)
	}
	if !m.changeCalled {
		t.Fatal("expected change to be called")
	}
}

func TestVerify(t *testing.T) {
	m := &mockR53{hostedZones: []r53types.HostedZone{zone("example.com")}}
	p := newProviderWithClient(m)
	zones, err := p.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 1 || zones[0].Name != "example.com" {
		t.Errorf("got %+v", zones)
	}
}

func TestInitRegistration(t *testing.T) {
	_, err := entree.NewProvider("route53", entree.Credentials{AccessKey: "", SecretKey: "sk"})
	if err == nil || !strings.Contains(err.Error(), "AccessKey required") {
		t.Fatalf("expected AccessKey required, got %v", err)
	}
}
