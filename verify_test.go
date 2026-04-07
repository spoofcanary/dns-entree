package entree

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func startFakeDNS(t *testing.T, handler dns.HandlerFunc) (string, func()) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &dns.Server{PacketConn: pc, Handler: handler}
	done := make(chan struct{})
	go func() {
		_ = srv.ActivateAndServe()
		close(done)
	}()
	// Brief readiness wait.
	time.Sleep(10 * time.Millisecond)
	shutdown := func() {
		_ = srv.Shutdown()
		<-done
	}
	return pc.LocalAddr().String(), shutdown
}

func txtHandler(name, value string) dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		if len(r.Question) > 0 && r.Question[0].Qtype == dns.TypeTXT {
			rr := &dns.TXT{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
				Txt: []string{value},
			}
			_ = name
			m.Answer = append(m.Answer, rr)
		}
		_ = w.WriteMsg(m)
	}
}

func cnameHandler(target string) dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		if len(r.Question) > 0 && r.Question[0].Qtype == dns.TypeCNAME {
			rr := &dns.CNAME{
				Hdr:    dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60},
				Target: dns.Fqdn(target),
			}
			m.Answer = append(m.Answer, rr)
		}
		_ = w.WriteMsg(m)
	}
}

func emptyHandler() dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		_ = w.WriteMsg(m)
	}
}

// withFakeNS swaps the package seams to point Verify at a fake authoritative server.
func withFakeNS(t *testing.T, addr string) {
	t.Helper()
	origLookup := lookupNS
	origAddr := nsAddrFunc
	lookupNS = func(string) ([]*net.NS, error) {
		return []*net.NS{{Host: "fake.ns."}}, nil
	}
	nsAddrFunc = func(_ context.Context, _ string) (string, error) {
		return addr, nil
	}
	t.Cleanup(func() {
		lookupNS = origLookup
		nsAddrFunc = origAddr
	})
}

func withRecursive(t *testing.T, addr string) {
	t.Helper()
	orig := recursiveAddr
	recursiveAddr = addr
	t.Cleanup(func() { recursiveAddr = orig })
}

func TestVerify_Authoritative_Match(t *testing.T) {
	addr, shutdown := startFakeDNS(t, txtHandler("_dmarc.example.com.", "v=DMARC1; p=none"))
	t.Cleanup(shutdown)
	withFakeNS(t, addr)
	withRecursive(t, addr) // never reached, but keep hermetic

	res, err := Verify(context.Background(), "example.com", VerifyOpts{
		RecordType: "TXT", Name: "_dmarc.example.com", Contains: "v=DMARC1",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Verified {
		t.Fatalf("expected Verified=true, got %+v", res)
	}
	if res.Method != "authoritative" {
		t.Errorf("method = %s", res.Method)
	}
	if !strings.Contains(res.CurrentValue, "v=DMARC1") {
		t.Errorf("current value = %s", res.CurrentValue)
	}
}

func TestVerify_Authoritative_NoMatch_FallbackToRecursive(t *testing.T) {
	authAddr, sh1 := startFakeDNS(t, txtHandler("_dmarc.example.com.", "v=spf1 -all"))
	t.Cleanup(sh1)
	recAddr, sh2 := startFakeDNS(t, txtHandler("_dmarc.example.com.", "v=DMARC1; p=reject"))
	t.Cleanup(sh2)

	withFakeNS(t, authAddr)
	withRecursive(t, recAddr)

	res, err := Verify(context.Background(), "example.com", VerifyOpts{
		RecordType: "TXT", Name: "_dmarc.example.com", Contains: "v=DMARC1",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Verified || res.Method != "recursive_fallback" {
		t.Fatalf("expected recursive_fallback verified, got %+v", res)
	}
}

func TestVerify_ContainsEmpty(t *testing.T) {
	addr, shutdown := startFakeDNS(t, txtHandler("x.example.com.", "anything"))
	t.Cleanup(shutdown)
	withFakeNS(t, addr)
	withRecursive(t, addr)

	res, err := Verify(context.Background(), "example.com", VerifyOpts{
		RecordType: "TXT", Name: "x.example.com",
	})
	if err != nil || !res.Verified {
		t.Fatalf("expected verified, got %+v err=%v", res, err)
	}
}

func TestVerify_CNAME(t *testing.T) {
	addr, shutdown := startFakeDNS(t, cnameHandler("target.example.net"))
	t.Cleanup(shutdown)
	withFakeNS(t, addr)
	withRecursive(t, addr)

	res, err := Verify(context.Background(), "example.com", VerifyOpts{
		RecordType: "CNAME", Name: "alias.example.com", Contains: "target",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Verified || !strings.Contains(res.CurrentValue, "target.example.net") {
		t.Fatalf("got %+v", res)
	}
}

func TestVerify_UnknownType(t *testing.T) {
	_, err := Verify(context.Background(), "example.com", VerifyOpts{
		RecordType: "ZZZ", Name: "example.com",
	})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestVerify_NoAnswers(t *testing.T) {
	addr, shutdown := startFakeDNS(t, emptyHandler())
	t.Cleanup(shutdown)
	withFakeNS(t, addr)
	withRecursive(t, addr)

	res, err := Verify(context.Background(), "example.com", VerifyOpts{
		RecordType: "TXT", Name: "_dmarc.example.com", Contains: "v=DMARC1",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Verified {
		t.Fatalf("expected not verified, got %+v", res)
	}
}
