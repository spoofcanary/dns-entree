package migrate

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// fakeZone holds an in-memory authoritative zone for testing.
type fakeZone struct {
	domain   string
	records  []dns.RR
	allowAXFR bool
}

func (z *fakeZone) handler() dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true

		if len(r.Question) == 0 {
			_ = w.WriteMsg(m)
			return
		}
		q := r.Question[0]

		if q.Qtype == dns.TypeAXFR {
			if !z.allowAXFR {
				m.Rcode = dns.RcodeRefused
				_ = w.WriteMsg(m)
				return
			}
			soa := &dns.SOA{
				Hdr: dns.RR_Header{Name: dns.Fqdn(z.domain), Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 3600},
				Ns:  "ns1." + dns.Fqdn(z.domain), Mbox: "h." + dns.Fqdn(z.domain),
				Serial: 1, Refresh: 7200, Retry: 3600, Expire: 1209600, Minttl: 3600,
			}
			tr := new(dns.Transfer)
			env := []*dns.Envelope{{RR: append(append([]dns.RR{soa}, z.records...), soa)}}
			ch := make(chan *dns.Envelope, 1)
			ch <- env[0]
			close(ch)
			_ = tr.Out(w, r, ch)
			return
		}

		// Standard query: filter records by name + type.
		for _, rr := range z.records {
			h := rr.Header()
			if !strings.EqualFold(h.Name, q.Name) {
				continue
			}
			if h.Rrtype == q.Qtype {
				m.Answer = append(m.Answer, rr)
			}
		}
		if len(m.Answer) == 0 {
			m.Rcode = dns.RcodeNameError
		}
		_ = w.WriteMsg(m)
	}
}

func startFakeAuth(t *testing.T, z *fakeZone) (string, func()) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", pc.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	h := z.handler()
	udp := &dns.Server{PacketConn: pc, Handler: h}
	tcp := &dns.Server{Listener: ln, Handler: h}
	go func() { _ = udp.ActivateAndServe() }()
	go func() { _ = tcp.ActivateAndServe() }()
	time.Sleep(20 * time.Millisecond)
	stop := func() {
		_ = udp.Shutdown()
		_ = tcp.Shutdown()
	}
	return pc.LocalAddr().String(), stop
}

func mustRR(t *testing.T, s string) dns.RR {
	t.Helper()
	rr, err := dns.NewRR(s)
	if err != nil {
		t.Fatal(err)
	}
	return rr
}

func TestScrape_AXFRSuccess(t *testing.T) {
	z := &fakeZone{
		domain:    "example.com",
		allowAXFR: true,
		records: []dns.RR{
			mustRR(t, "example.com. 3600 IN A 192.0.2.1"),
			mustRR(t, "www.example.com. 3600 IN A 192.0.2.2"),
			mustRR(t, "_dmarc.example.com. 3600 IN TXT \"v=DMARC1; p=none\""),
			// Out-of-zone: must be dropped.
			mustRR(t, "evil.attacker.com. 3600 IN A 6.6.6.6"),
		},
	}
	addr, stop := startFakeAuth(t, z)
	defer stop()

	got, err := ScrapeZone(context.Background(), "example.com", ScrapeOptions{
		Nameservers: []string{addr},
	})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if got.Source != "axfr" {
		t.Errorf("source = %s", got.Source)
	}
	if len(got.Records) != 3 {
		t.Errorf("want 3 records, got %d: %+v", len(got.Records), got.Records)
	}
	foundWarn := false
	for _, w := range got.Warnings {
		if strings.Contains(w, "out-of-zone") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected out-of-zone warning, got %v", got.Warnings)
	}
}

func TestScrape_AXFRRefused_FallsBackToIterated(t *testing.T) {
	z := &fakeZone{
		domain:    "example.com",
		allowAXFR: false,
		records: []dns.RR{
			mustRR(t, "example.com. 3600 IN A 192.0.2.1"),
			mustRR(t, "www.example.com. 3600 IN A 192.0.2.2"),
			mustRR(t, "_dmarc.example.com. 3600 IN TXT \"v=DMARC1; p=none\""),
		},
	}
	addr, stop := startFakeAuth(t, z)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := ScrapeZone(ctx, "example.com", ScrapeOptions{
		Nameservers:      []string{addr},
		OnlyLabels:       []string{"@", "www", "_dmarc"},
		QueriesPerSecond: 500,
	})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if got.Source != "iterated" {
		t.Errorf("source = %s", got.Source)
	}
	types := map[string]int{}
	for _, r := range got.Records {
		types[r.Type+"|"+r.Name]++
	}
	if types["A|example.com"] == 0 || types["A|www.example.com"] == 0 || types["TXT|_dmarc.example.com"] == 0 {
		t.Errorf("missing expected records: %+v", got.Records)
	}
}

func TestScrape_IteratedCNAMEEnumeration(t *testing.T) {
	z := &fakeZone{
		domain:    "example.com",
		allowAXFR: false,
		records: []dns.RR{
			mustRR(t, "www.example.com. 3600 IN CNAME app.example.com."),
			mustRR(t, "app.example.com. 3600 IN A 192.0.2.10"),
		},
	}
	addr, stop := startFakeAuth(t, z)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := ScrapeZone(ctx, "example.com", ScrapeOptions{
		Nameservers:      []string{addr},
		OnlyLabels:       []string{"www"},
		QueriesPerSecond: 500,
	})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	var sawCNAME, sawA bool
	for _, r := range got.Records {
		if r.Type == "CNAME" && r.Name == "www.example.com" {
			sawCNAME = true
		}
		if r.Type == "A" && r.Name == "app.example.com" {
			sawA = true
		}
	}
	if !sawCNAME || !sawA {
		t.Errorf("CNAME enumeration failed: cname=%v a=%v records=%+v", sawCNAME, sawA, got.Records)
	}
}
