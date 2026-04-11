// detect-and-apply-template detects the DNS provider for a domain, loads a
// Domain Connect template from the cached Domain-Connect/Templates repo, and
// applies it via a Cloudflare PushService.
//
// Prereqs:
//
//	export DNSENTREE_CLOUDFLARE_TOKEN=...
//
// Run:
//
//	go run . example.com exampleservice.com template1
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/providers/cloudflare"
	"github.com/spoofcanary/dns-entree/template"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("usage: detect-and-apply-template <domain> <providerID> <serviceID>")
	}
	domain := os.Args[1]
	providerID := os.Args[2]
	serviceID := os.Args[3]

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	det, err := entree.DetectProvider(ctx, domain)
	if err != nil {
		log.Fatalf("detect: %v", err)
	}
	fmt.Printf("detected: provider=%s label=%q supported=%v method=%s\n",
		det.Provider, det.Label, det.Supported, det.Method)

	if det.Provider != entree.ProviderCloudflare {
		log.Fatalf("this example only wires Cloudflare; got %s", det.Provider)
	}

	token := os.Getenv("DNSENTREE_CLOUDFLARE_TOKEN")
	if token == "" {
		log.Fatalf("DNSENTREE_CLOUDFLARE_TOKEN not set")
	}
	prov, err := cloudflare.NewProvider(token)
	if err != nil {
		log.Fatalf("cloudflare provider: %v", err)
	}
	pushSvc := entree.NewPushService(prov)

	// Sync git cache, then load the requested template.
	if err := template.SyncTemplates(ctx); err != nil {
		log.Fatalf("sync templates: %v", err)
	}
	tmpl, err := template.LoadTemplate(ctx, providerID, serviceID)
	if err != nil {
		log.Fatalf("load template: %v", err)
	}

	vars := map[string]string{"domain": domain}
	results, err := template.ApplyTemplate(ctx, pushSvc, domain, tmpl, vars)
	if err != nil {
		log.Printf("apply (with partial errors): %v", err)
	}
	for _, r := range results {
		fmt.Printf("  %s %s -> %s (verified=%v)\n", r.Status, r.RecordName, r.RecordValue, r.Verified)
	}
}
