package main

// Fake migrate adapter registered under slug "fake" for CLI integration tests.
// Inert in production: the "fake" provider slug is only reachable via explicit
// --to fake. Mirrors fakeprovider.go's approach.

import (
	"context"

	"github.com/spoofcanary/dns-entree/migrate"
)

type fakeAdapter struct{}

func (fakeAdapter) EnsureZone(ctx context.Context, domain string, opts migrate.ProviderOpts) (migrate.ZoneInfo, error) {
	return migrate.ZoneInfo{
		ZoneID:      "fake-zone-id",
		Nameservers: []string{"ns1.fake.example.", "ns2.fake.example."},
		Created:     true,
	}, nil
}

func init() {
	migrate.RegisterAdapter("fake", fakeAdapter{})
}
