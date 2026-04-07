// push-dkim pushes a DKIM CNAME via AWS Route 53.
//
// Prereqs:
//
//	export DNSENTREE_AWS_ACCESS_KEY_ID=...
//	export DNSENTREE_AWS_SECRET_ACCESS_KEY=...
//	export DNSENTREE_AWS_REGION=us-east-1
//
// Run:
//
//	go run . example.com selector1 selector1.dkim.provider.com
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/providers/route53"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("usage: push-dkim <domain> <selector> <target>")
	}
	domain := os.Args[1]
	selector := os.Args[2]
	target := os.Args[3]

	ak := os.Getenv("DNSENTREE_AWS_ACCESS_KEY_ID")
	sk := os.Getenv("DNSENTREE_AWS_SECRET_ACCESS_KEY")
	region := os.Getenv("DNSENTREE_AWS_REGION")
	if ak == "" || sk == "" {
		log.Fatalf("DNSENTREE_AWS_ACCESS_KEY_ID / DNSENTREE_AWS_SECRET_ACCESS_KEY not set")
	}
	if region == "" {
		region = "us-east-1"
	}

	prov, err := route53.NewProvider(ak, sk, region)
	if err != nil {
		log.Fatalf("route53 provider: %v", err)
	}

	svc := entree.NewPushService(prov)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	name := selector + "._domainkey." + domain
	res, err := svc.PushCNAMERecord(ctx, domain, name, target)
	if err != nil {
		log.Fatalf("push: %v", err)
	}
	fmt.Printf("status=%s verified=%v value=%q\n", res.Status, res.Verified, res.RecordValue)
}
