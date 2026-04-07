// push-dmarc pushes a DMARC TXT record via Cloudflare.
//
// Prereqs:
//
//	export DNSENTREE_CLOUDFLARE_TOKEN=...
//
// Run:
//
//	go run . example.com dmarc@example.com
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/providers/cloudflare"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("usage: push-dmarc <domain> <rua-email>")
	}
	domain := os.Args[1]
	rua := os.Args[2]

	token := os.Getenv("DNSENTREE_CLOUDFLARE_TOKEN")
	if token == "" {
		log.Fatalf("DNSENTREE_CLOUDFLARE_TOKEN not set")
	}

	prov, err := cloudflare.NewProvider(token)
	if err != nil {
		log.Fatalf("cloudflare provider: %v", err)
	}

	svc := entree.NewPushService(prov)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	policy := fmt.Sprintf("v=DMARC1; p=none; rua=mailto:%s", rua)
	res, err := svc.PushTXTRecord(ctx, domain, "_dmarc."+domain, policy)
	if err != nil {
		log.Fatalf("push: %v", err)
	}
	fmt.Printf("status=%s verified=%v value=%q\n", res.Status, res.Verified, res.RecordValue)
}
