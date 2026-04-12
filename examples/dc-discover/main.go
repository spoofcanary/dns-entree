//go:build ignore

// dc-discover performs Domain Connect v2 discovery for a domain and prints
// the provider settings.
//
// Run:
//
//	go run . example.com
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spoofcanary/dns-entree/domainconnect"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: dc-discover <domain>")
	}
	domain := os.Args[1]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := domainconnect.Discover(ctx, domain)
	if err != nil {
		log.Fatalf("discover: %v", err)
	}
	if !res.Supported {
		fmt.Println("Domain Connect: not supported")
		return
	}
	fmt.Printf("providerId:   %s\n", res.ProviderID)
	fmt.Printf("providerName: %s\n", res.ProviderName)
	fmt.Printf("urlSyncUX:    %s\n", res.URLSyncUX)
	fmt.Printf("urlAsyncUX:   %s\n", res.URLAsyncUX)
	fmt.Printf("urlAPI:       %s\n", res.URLAPI)
	fmt.Printf("width/height: %dx%d\n", res.Width, res.Height)
}
