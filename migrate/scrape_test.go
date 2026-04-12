package migrate

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// fakeZone holds an in-memory authoritative zone for testing.
type fakeZone struct {
	mu        sync.RWMutex
	domain    string
	records   []dns.RR
	allowAXFR bool
}

// addRecord appends a record to the zone safely.
func (z *fakeZone) addRecord(rr dns.RR) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.records = append(z.records, rr)
}

// snapshot returns a copy of the current records for safe iteration.
func (z *fakeZone) snapshot() []dns.RR {
	z.mu.RLock()
	defer z.mu.RUnlock()
	cp := make([]dns.RR, len(z.records))
	copy(cp, z.records)
	return cp
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
			snap := z.snapshot()
			env := []*dns.Envelope{{RR: append(append([]dns.RR{soa}, snap...), soa)}}
			ch := make(chan *dns.Envelope, 1)
			ch <- env[0]
			close(ch)
			_ = tr.Out(w, r, ch)
			return
		}

		// Standard query: filter records by name + type.
		snap := z.snapshot()
		for _, rr := range snap {
			h := rr.Header()
			if !strings.EqualFold(h.Name, q.Name) {
				continue
			}
			if h.Rrtype == q.Qtype {
				m.Answer = append(m.Answer, dns.Copy(rr))
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

// truncatedAXFRServer serves an AXFR that contains only the opening SOA
// plus some records but NO closing SOA — simulating a TCP connection cut
// mid-stream.
func startTruncatedAXFR(t *testing.T, domain string) (string, func()) {
	t.Helper()
	h := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 0 || r.Question[0].Qtype != dns.TypeAXFR {
			m := new(dns.Msg)
			m.SetReply(r)
			m.Rcode = dns.RcodeRefused
			_ = w.WriteMsg(m)
			return
		}
		soa := &dns.SOA{
			Hdr:    dns.RR_Header{Name: dns.Fqdn(domain), Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 3600},
			Ns:     "ns1." + dns.Fqdn(domain),
			Mbox:   "h." + dns.Fqdn(domain),
			Serial: 1, Refresh: 7200, Retry: 3600, Expire: 1209600, Minttl: 3600,
		}
		a := mustRR(t, domain+". 3600 IN A 192.0.2.77")
		// Write a single DNS message with opening SOA + one A but NO closing
		// SOA. miekg/dns Transfer.In treats this as one envelope with
		// soaCount=1 and returns without error, exercising our completeness
		// check.
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		m.Answer = []dns.RR{soa, a}
		_ = w.WriteMsg(m)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	pc, err := net.ListenPacket("udp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	udp := &dns.Server{PacketConn: pc, Handler: h}
	tcp := &dns.Server{Listener: ln, Handler: h}
	go func() { _ = udp.ActivateAndServe() }()
	go func() { _ = tcp.ActivateAndServe() }()
	time.Sleep(20 * time.Millisecond)
	return ln.Addr().String(), func() {
		_ = udp.Shutdown()
		_ = tcp.Shutdown()
	}
}

func TestScrape_AXFRTruncated_MissingClosingSOA(t *testing.T) {
	addr, stop := startTruncatedAXFR(t, "example.com")
	defer stop()

	_, err := axfrTransfer(context.Background(), "example.com", addr, 2*time.Second)
	if err == nil {
		t.Fatal("expected error for truncated AXFR, got nil")
	}
	if !strings.Contains(err.Error(), "axfr incomplete") {
		t.Errorf("error = %q, want 'axfr incomplete'", err.Error())
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
