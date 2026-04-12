package entree

import (
	"strings"
	"testing"
)

func FuzzValidateDNSName(f *testing.F) {
	f.Add("example.com")
	f.Add("_dmarc.example.com")
	f.Add("*.example.com")
	f.Add("")
	f.Add(strings.Repeat("a", 64) + ".com")
	f.Add("example.com.")
	f.Add("\x00.com")
	f.Add("xn--n3h.com") // punycode emoji domain
	f.Add("a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z")
	f.Add("-leading.com")
	f.Add("trailing-.com")

	f.Fuzz(func(t *testing.T, name string) {
		// Must not panic.
		_ = ValidateDNSName(name)
	})
}
