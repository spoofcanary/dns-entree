// spf-merge is the library equivalent of `entree spf-merge`. Pure, no I/O.
//
// Run:
//
//	go run . "v=spf1 include:_spf.google.com ~all" servers.mcsv.net
package main

import (
	"fmt"
	"log"
	"os"

	entree "github.com/spoofcanary/dns-entree"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("usage: spf-merge <current-spf> <include> [<include>...]")
	}
	current := os.Args[1]
	includes := os.Args[2:]

	res, err := entree.MergeSPF(current, includes)
	if err != nil {
		log.Fatalf("merge: %v", err)
	}
	fmt.Println("value:", res.Value)
	fmt.Println("changed:", res.Changed)
	fmt.Println("lookup_count:", res.LookupCount)
	fmt.Println("lookup_limit_exceeded:", res.LookupLimitExceeded)
	for _, w := range res.Warnings {
		fmt.Println("warning:", w)
	}
}
