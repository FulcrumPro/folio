// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"sort"
	"strings"
)

// RenderCSSPropertiesMarkdown produces the contents of docs/CSS_SUPPORT.md
// from the cssProperties registry. The output is grouped by Category in
// a fixed order, then alphabetized within each category. Properties in
// the registry are the ground truth — any change to cssProperties
// regenerates the doc on the next `go generate`.
//
// The function is exported so that internal/gen-css-docs/main.go can
// invoke it without importing unexported internals. It is otherwise not
// part of Folio's public API contract — callers outside the doc
// generator should not rely on its output format.
func RenderCSSPropertiesMarkdown() string {
	var b strings.Builder

	b.WriteString("# Folio CSS support\n\n")
	b.WriteString("> Auto-generated from `html/css_props.go`. Do not edit by hand.\n")
	b.WriteString("> Run `go generate ./html/...` to regenerate after changing the registry.\n\n")
	b.WriteString("Folio's HTML-to-PDF converter recognizes the CSS properties listed below.\n")
	b.WriteString("Properties not in this document are silently ignored at render time.\n\n")

	// Sort properties by Category, then by Name. Categories appear in a
	// fixed display order so the doc is stable across runs.
	categoryOrder := []string{
		"Typography",
		"Color",
		"Backgrounds",
		"BoxModel",
		"Borders",
		"Layout",
		"Flexbox",
		"Grid",
		"MultiColumn",
		"Tables",
		"Pagination",
		"Lists",
		"Effects",
		"PDF",
	}

	byCat := make(map[string][]cssProperty)
	for _, p := range cssProperties {
		byCat[p.Category] = append(byCat[p.Category], p)
	}
	for cat := range byCat {
		sort.Slice(byCat[cat], func(i, j int) bool {
			return byCat[cat][i].Name < byCat[cat][j].Name
		})
	}

	// Summary count.
	b.WriteString("## At a glance\n\n")
	b.WriteString("| Category | Properties |\n")
	b.WriteString("|---|---:|\n")
	totalCount := 0
	for _, cat := range categoryOrder {
		entries := byCat[cat]
		if len(entries) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("| %s | %d |\n", cat, len(entries)))
		totalCount += len(entries)
	}
	b.WriteString(fmt.Sprintf("| **Total** | **%d** |\n\n", totalCount))

	// Per-category tables.
	for _, cat := range categoryOrder {
		entries := byCat[cat]
		if len(entries) == 0 {
			continue
		}
		b.WriteString("## ")
		b.WriteString(cat)
		b.WriteString("\n\n")
		b.WriteString("| Property | Aliases | Accepted values | Notes |\n")
		b.WriteString("|---|---|---|---|\n")
		for _, p := range entries {
			aliases := "—"
			if len(p.Aliases) > 0 {
				aliases = "`" + strings.Join(p.Aliases, "`, `") + "`"
			}
			values := "—"
			if len(p.Values) > 0 {
				escaped := make([]string, len(p.Values))
				for i, v := range p.Values {
					escaped[i] = "`" + v + "`"
				}
				values = strings.Join(escaped, ", ")
			}
			notes := p.Notes
			if notes == "" {
				notes = "—"
			}
			b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", p.Name, aliases, values, notes))
		}
		b.WriteString("\n")
	}

	// Known unsupported list — hardcoded for now; future work could
	// derive this from a separate registry.
	b.WriteString("## Known unsupported features\n\n")
	b.WriteString("These properties are commonly requested but NOT supported by Folio's HTML converter:\n\n")
	b.WriteString("| Feature | Workaround |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `oklch()` color | Use precomputed hex equivalents. Folio renders sRGB only. |\n")
	b.WriteString("| `color-mix()` | Precompute the mixed color or define a CSS variable with the result. |\n")
	b.WriteString("| `-webkit-line-clamp` / `line-clamp` | Truncate at the template / runtime layer before HTML emission; PDFs are paginated, not scrollable. |\n")
	b.WriteString("| `text-wrap: pretty` | Cosmetic only; render without it. |\n")
	b.WriteString("| ICC profiles | Folio renders into the sRGB color space. |\n")

	return b.String()
}

//go:generate go run ../internal/gen-css-docs
