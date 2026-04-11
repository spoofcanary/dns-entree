// Command entree is the reference CLI for the dns-entree library.
//
// It exposes the library's core operations as subcommands: detect, apply,
// verify, spf-merge, dc-discover, and templates. Credentials are loaded from
// a YAML-ish credentials file (see --credentials) or environment variables.
// Every command supports --json for machine-readable output so the CLI can be
// used as an agent-callable tool.
//
// Run `entree --help` for the full command tree, or see docs/cli.md in the
// repository for the generated reference.
//
// # Stability
//
// Stable. The command surface and flag names are covered by semver from
// v1.0.0 forward.
package main
