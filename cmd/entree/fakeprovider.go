package main

// The fake provider implementation lives in internal/fakeprovider so that the
// HTTP API package can share it. Importing the package for its side effect
// (init registration) keeps the CLI binary's "fake" slug working unchanged.

import (
	_ "github.com/spoofcanary/dns-entree/internal/fakeprovider"
)
