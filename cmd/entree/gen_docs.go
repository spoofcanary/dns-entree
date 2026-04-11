//go:build docs

package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// This file is built only with `-tags=docs`. It overrides main() to generate
// a single-file Markdown reference for the entree CLI at docs/cli.md.
func init() {
	// Replace the normal main entrypoint behavior by exiting from a custom
	// runner. We register via a sentinel: when DOCS_GEN=1, generate and exit.
	if os.Getenv("ENTREE_GEN_DOCS") != "1" {
		return
	}
	out := "docs/cli.md"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := generateSingleFileDocs(rootCmd, out); err != nil {
		fmt.Fprintln(os.Stderr, "gen-docs:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func generateSingleFileDocs(root *cobra.Command, path string) error {
	root.DisableAutoGenTag = true
	var buf bytes.Buffer
	buf.WriteString("# entree CLI Reference\n\n")
	buf.WriteString("Auto-generated from cobra command definitions. Do not edit by hand.\n")
	buf.WriteString("Regenerate with `make docs`.\n\n")
	if err := walkAndRender(root, &buf); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func walkAndRender(cmd *cobra.Command, buf *bytes.Buffer) error {
	if !cmd.IsAvailableCommand() && cmd.Name() != "entree" {
		return nil
	}
	var sub bytes.Buffer
	if err := doc.GenMarkdown(cmd, &sub); err != nil {
		return err
	}
	// Strip cobra's "### SEE ALSO" footer to keep the single-file doc clean.
	out := stripSeeAlso(sub.String())
	buf.WriteString(out)
	buf.WriteString("\n---\n\n")

	children := cmd.Commands()
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
	for _, c := range children {
		if c.Name() == "help" || c.Hidden {
			continue
		}
		if err := walkAndRender(c, buf); err != nil {
			return err
		}
	}
	return nil
}

func stripSeeAlso(s string) string {
	idx := bytes.Index([]byte(s), []byte("### SEE ALSO"))
	if idx < 0 {
		return s
	}
	return s[:idx]
}
