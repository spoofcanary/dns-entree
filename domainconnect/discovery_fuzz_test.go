package domainconnect

import (
	"context"
	"strings"
	"testing"
	"time"
)

func FuzzDiscoverDomainConnect(f *testing.F) {
	f.Add("example.com")
	f.Add("sub.example.com")
	f.Add("")
	f.Add("127.0.0.1")
	f.Add(strings.Repeat("a", 300) + ".com")
	f.Add("example.com\x00evil.com") // null byte injection
	f.Add("xn--n3h.com")             // punycode
	f.Add("*.example.com")
	f.Add(".example.com")
	f.Add("example..com")

	f.Fuzz(func(t *testing.T, domain string) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		// Must not panic. Errors are acceptable.
		_, _ = Discover(ctx, domain)
	})
}
