// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Command gen-css-docs writes docs/CSS_SUPPORT.md from the
// html.cssProperties registry. Run via `go generate ./html/...` after
// changing the registry. The output is checked into version control;
// CI fails if the file drifts from what the generator would produce.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/carlos7ags/folio/html"
)

func main() {
	// The generator runs from html/ (per the go:generate directive's
	// ../internal/gen-css-docs path). Walk up to the repo root by
	// looking for go.mod so the output path is stable regardless of
	// where the user invoked go generate.
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-css-docs: %v\n", err)
		os.Exit(1)
	}
	out := filepath.Join(root, "docs", "CSS_SUPPORT.md")

	content := html.RenderCSSPropertiesMarkdown()

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gen-css-docs: mkdir docs/: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gen-css-docs: write %s: %v\n", out, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes)\n", out, len(content))
}

// findRepoRoot walks up from the current working directory looking for
// the directory containing go.mod. Returns an error if not found
// within a reasonable depth.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 16; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found in any parent of cwd")
}
