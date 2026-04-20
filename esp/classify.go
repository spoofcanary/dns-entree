package esp

import (
	"context"
	"strings"
	"time"

	entree "github.com/spoofcanary/dns-entree"
)

// ClassifyDomain runs the full classification pipeline for a domain:
// recursive SPF walk + DKIM CNAME probe + result merge. It performs DNS
// lookups and therefore requires a non-nil context that is not already
// cancelled.
//
// If spfRecord is empty, only DKIM-based classification is returned. If no
// DKIM selectors are supplied, DefaultDKIMSelectors is probed.
//
// Results are deduplicated by (Name, Infrastructure) so that a sender
// detected via both SPF and DKIM appears once (with both SPFSource and
// DKIMSelector populated).
func ClassifyDomain(
	ctx context.Context,
	domain string,
	spfRecord string,
	dkimSelectors []string,
) []SenderClassification {
	spfResolver := entree.NewNetSPFResolver(2 * time.Second)
	dkimResolver := NewNetDKIMResolver(2 * time.Second)
	return ClassifyDomainWithResolvers(ctx, domain, spfRecord, dkimSelectors, spfResolver, dkimResolver)
}

// ClassifyDomainWithResolvers is ClassifyDomain with injectable resolvers.
// Use for tests and for environments that require custom DNS plumbing.
func ClassifyDomainWithResolvers(
	ctx context.Context,
	domain string,
	spfRecord string,
	dkimSelectors []string,
	spfResolver entree.SPFResolver,
	dkimResolver DKIMResolver,
) []SenderClassification {
	spfResults := ClassifyFromSPFRecursive(ctx, spfResolver, spfRecord)
	dkimResults := ClassifyFromDKIM(ctx, dkimResolver, domain, dkimSelectors)
	return Merge(spfResults, dkimResults)
}

// Merge combines SPF-derived and DKIM-derived classifications, preferring
// richer entries (those with both SPFSource and DKIMSelector populated).
// Dedup key is (Name, Infrastructure). SPF entries seed the map; DKIM
// entries merge into matching entries or append as new rows.
func Merge(spfResults, dkimResults []SenderClassification) []SenderClassification {
	type key struct {
		name string
		infra Infrastructure
	}
	index := make(map[key]*SenderClassification, len(spfResults)+len(dkimResults))
	order := make([]key, 0, len(spfResults)+len(dkimResults))

	add := func(c SenderClassification) {
		k := key{name: strings.ToLower(c.Name), infra: c.Infrastructure}
		if existing, ok := index[k]; ok {
			if existing.SPFSource == "" && c.SPFSource != "" {
				existing.SPFSource = c.SPFSource
				existing.ViaChain = c.ViaChain
			}
			if existing.DKIMSelector == "" && c.DKIMSelector != "" {
				existing.DKIMSelector = c.DKIMSelector
				existing.DKIMTarget = c.DKIMTarget
			}
			return
		}
		cc := c
		index[k] = &cc
		order = append(order, k)
	}
	for _, r := range spfResults {
		add(r)
	}
	for _, r := range dkimResults {
		add(r)
	}
	out := make([]SenderClassification, 0, len(order))
	for _, k := range order {
		out = append(out, *index[k])
	}
	return out
}
