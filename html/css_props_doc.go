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
	b.WriteString("> Auto-generated from `html/css_props.go` and `html/css.go`. Do not edit by hand.\n")
	b.WriteString("> Run `go generate ./html/...` to regenerate after changing the registry.\n\n")
	b.WriteString("Folio's HTML-to-PDF converter recognizes the CSS properties listed below.\n")
	b.WriteString("Properties not in this document are silently ignored at render time.\n\n")

	// Orientation paragraph: tells a first-time evaluator how to read
	// the rest of the document.
	b.WriteString("## How to read this document\n\n")
	b.WriteString("Each per-category table lists the property name, any alternative names\n")
	b.WriteString("(aliases) accepted by the parser, the value forms that are recognized,\n")
	b.WriteString("and any notes about parsing or interactions with other properties.\n\n")
	b.WriteString("Value forms are written in CSS spec shorthand: `<length>` means a\n")
	b.WriteString("length value (e.g. `12px`, `1em`, `0.5in`); `<color>` means any\n")
	b.WriteString("supported color form (named, hex, rgb/rgba, hsl/hsla, cmyk); and so on.\n")
	b.WriteString("See [Value-form glossary](#value-form-glossary) below for the full list.\n\n")
	b.WriteString("If you don't see a property here, Folio's parser silently ignores it\n")
	b.WriteString("at render time — there is no warning. Use\n")
	b.WriteString("[`html.Options.StrictAssets`](../html) to escalate certain asset failures,\n")
	b.WriteString("but unknown CSS properties are always silent.\n\n")

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
		fmt.Fprintf(&b, "| %s | %d |\n", cat, len(entries))
		totalCount += len(entries)
	}
	fmt.Fprintf(&b, "| **Total** | **%d** |\n\n", totalCount)

	// Value-form glossary — disambiguates the angle-bracket placeholders
	// that appear in per-property "Accepted values" columns.
	b.WriteString("## Value-form glossary\n\n")
	b.WriteString("Angle-bracket placeholders used in the per-property tables below.\n\n")
	b.WriteString("| Placeholder | Meaning |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `<length>` | A CSS length: `<number><unit>` where unit is `px`, `pt`, `em`, `rem`, `cm`, `mm`, or `in`. Examples: `12px`, `1.5em`, `0.5in`. Distinct from `<percentage>`, which is listed separately as an alternative in per-property tables. |\n")
	b.WriteString("| `<percentage>` | A `<number>%`. Resolves against the containing context (line-height, parent dimension, etc.). |\n")
	b.WriteString("| `<number>` | A unitless real number, e.g. `1.5`, `0.7`, `-2`. |\n")
	b.WriteString("| `<integer>` | A whole number, e.g. `0`, `5`, `-1`. Range constraints (e.g. `<integer 1..6>`) are listed in the per-property table. |\n")
	b.WriteString("| `<string>` | A quoted text literal, e.g. `\"My Title\"` or `'caption'`. |\n")
	b.WriteString("| `<color>` | Any of: `<named>` (`red`, `transparent`), `<hex>` (`#abc`, `#aabbcc`), `rgb()`, `rgba()`, `hsl()`, `hsla()`, `cmyk()`. Folio renders sRGB only — `oklch()` and `color-mix()` are not supported. |\n")
	b.WriteString("| `<named>`, `<hex>` | Component forms of `<color>`: `<named>` is a CSS named color (`red`, `aliceblue`, etc.); `<hex>` is `#RGB`, `#RGBA`, `#RRGGBB`, or `#RRGGBBAA`. |\n")
	b.WriteString("| `<line-width>` | A `<length>` or one of the keywords `thin`, `medium`, `thick`. Used in border/outline shorthands. |\n")
	b.WriteString("| `<line-style>` | One of `solid`, `dashed`, `dotted`, `double`, `none`. |\n")
	b.WriteString("| `<position>` | A 1- or 2-component position keyword/length. Examples: `center`, `top right`, `50% 25%`, `10px 20px`. Applies to `background-position`, `object-position`, `transform-origin`. |\n")
	b.WriteString("| `<grid-line>` | A grid line reference: an integer (e.g. `2`), a `span` keyword (`span 3`), or a named line (rare; line names not yet supported). |\n")
	b.WriteString("| `<track-list>` | A space-separated list of track sizes for `grid-template-columns`/`-rows`. Examples: `1fr 1fr`, `100px auto`, `repeat(3, 1fr)`. |\n")
	b.WriteString("| `<track-size>` | A single grid track size: `<length>`, `<percentage>`, `<number>fr`, `auto`, `min-content`, `max-content`. |\n")
	b.WriteString("| `<ratio>` | An aspect ratio expressed as `<number>/<number>` or a single `<number>`. Example: `16/9`. |\n")
	b.WriteString("| `<gradient>` | `linear-gradient(...)`, `repeating-linear-gradient(...)`, `radial-gradient(...)`, or `repeating-radial-gradient(...)`. |\n")
	b.WriteString("| `<transform-function>` | A CSS transform: `translate()`, `translateX()`/`Y()`, `rotate()`, `scale()`/`X()`/`Y()`, `skew()`/`X()`/`Y()`. |\n")
	b.WriteString("| `<offset-x>`, `<offset-y>`, `<blur>`, `<spread>` | Component lengths in shadow shorthands (`box-shadow`, `text-shadow`). All are `<length>`; spread accepts negatives to inset the shadow. |\n")
	b.WriteString("| `<identifier>` | A custom name, e.g. for `counter-reset` or `string-set`. |\n")
	b.WriteString("\n")
	b.WriteString("**`calc()`, `min()`, `max()`, `clamp()`** are accepted everywhere a `<length>` or `<percentage>` is. The parser preserves them as single tokens through shorthand splitting.\n\n")

	// Box-alignment cross-context callout. align-items / justify-content
	// etc. are listed under Flexbox in the per-category tables, but in
	// CSS 3 Box Alignment they apply to Grid containers too. Note this
	// once at the top of the doc to avoid confusion.
	b.WriteString("## Box-alignment properties\n\n")
	b.WriteString("`justify-content`, `align-items`, `align-self`, and `align-content` are\n")
	b.WriteString("listed under Flexbox or Grid in the per-category tables for grouping,\n")
	b.WriteString("but per CSS Box Alignment Level 3 they apply to BOTH flex and grid\n")
	b.WriteString("containers. Folio honors them in either context.\n\n")
	b.WriteString("Similarly, `gap` (and its alias `grid-gap`) is grouped under Grid but\n")
	b.WriteString("also takes effect on flex containers as the gap between items.\n\n")

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
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", p.Name, aliases, values, notes)
		}
		b.WriteString("\n")
	}

	// At-rules — hand-curated section. The set of @-rules Folio
	// recognizes is small and changes rarely; a parallel registry would
	// be more bookkeeping than the doc payoff justifies. The
	// TestAtRulesDocCoverage CI guard parses html/css.go and asserts
	// every @-prefixed string literal in parseCSS is mentioned here, so
	// new at-rule support cannot land without updating this section.
	b.WriteString("## At-rules\n\n")
	b.WriteString("CSS at-rules recognized by Folio's stylesheet parser. Anything not listed here\n")
	b.WriteString("is silently dropped during parsing — there is no warning.\n\n")
	b.WriteString("| Rule | Selectors / context | Notes |\n")
	b.WriteString("|---|---|---|\n")
	b.WriteString("| `@font-face` | — | Declares a custom font face. Recognized descriptors: `font-family`, `src`, `font-weight`, `font-style`. The `format()` annotation in `src` is advisory; Folio inspects the URL contents to determine format (WOFF1, TTF, TTC). WOFF2 is not supported. |\n")
	b.WriteString("| `@page` | `:first`, `:left`, `:right`, no selector | Page-level styling: page size, margins, and nested margin boxes. Pseudo-selectors target the first page or left/right pages in a duplex flow. |\n")
	b.WriteString("| `@page` margin boxes | `@top-left`, `@top-center`, `@top-right`, `@bottom-left`, `@bottom-center`, `@bottom-right` | Running headers/footers, declared inside an `@page` block. Populate via static `content`, `string()`, or `counter(page)`. The four corner boxes (`@top-left-corner`, etc.) and the `@left-*` / `@right-*` boxes are not interpreted. |\n")
	b.WriteString("| `@supports` | `(<property>: <value>)`, `not (...)`, `and`, `or` | Feature query. Inner rules are parsed only if the condition evaluates true against Folio's actual support — useful for shipping fallbacks alongside Folio-specific styling. |\n")
	b.WriteString("| `@media print` | — | Treated as unconditional (PDF is a print medium). Inner rules are parsed as if at the top level. Other `@media` queries are silently discarded; see below. |\n\n")
	b.WriteString("### Silently ignored at-rules\n\n")
	b.WriteString("Listed for evaluators migrating from a browser-based renderer. None of\n")
	b.WriteString("these produce a warning — the rule and its body are dropped during parsing.\n\n")
	b.WriteString("| Rule | Why |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `@media screen`, `@media (max-width: ...)`, etc. | Only `@media print` is interpreted; PDF output has fixed page geometry, so viewport breakpoints have no analogue. |\n")
	b.WriteString("| `@import` | External stylesheet imports are not followed during CSS parsing. Use `<link rel=\"stylesheet\">` in the HTML instead — those are loaded through the asset resolver. |\n")
	b.WriteString("| `@keyframes`, `@-webkit-keyframes` | PDF has no animation timeline. |\n")
	b.WriteString("| `@counter-style` | Custom list counter styles are not parsed; only the keywords listed under `list-style-type` are recognized. |\n")
	b.WriteString("| `@namespace`, `@charset` | Not interpreted. |\n")
	b.WriteString("| `@layer`, `@scope`, `@container`, `@property` | Newer CSS spec features; not interpreted. |\n\n")

	// Known unsupported list — hardcoded for now; future work could
	// derive this from a separate registry.
	b.WriteString("## Known unsupported features\n\n")
	b.WriteString("These properties / values are commonly requested but NOT recognized by Folio.\n")
	b.WriteString("Folio silently ignores unknown property names, so a stylesheet that uses\n")
	b.WriteString("any of these will render — just without the styling those declarations\n")
	b.WriteString("would have applied in a browser.\n\n")
	b.WriteString("| Feature | Why | Workaround |\n")
	b.WriteString("|---|---|---|\n")
	b.WriteString("| `oklch()`, `oklab()`, `lch()`, `lab()` color | Folio renders sRGB only; no ICC profile support. | Precompute the sRGB equivalent and use `#hex` or `rgb()`. |\n")
	b.WriteString("| `color-mix()` | Folio's parser doesn't expand the function. | Precompute the mixed color, or assign it to a CSS variable: `--btn-tint: #c44;`. |\n")
	b.WriteString("| `-webkit-line-clamp` / `line-clamp` | PDFs are paginated, not scrollable; the property has no analogue. | Truncate before HTML emission, or use `layout.Paragraph.SplitAfterLine` for first-N-lines-plus-appendix flows. |\n")
	b.WriteString("| `text-wrap: pretty` / `text-wrap: balance` | Browser-only line-break heuristic; cosmetic. | Render without it. |\n")
	b.WriteString("| `filter`, `backdrop-filter`, `mix-blend-mode` | PDF lacks an analogue for screen-compositing. | Pre-bake effects into images. |\n")
	b.WriteString("| `:hover`, `:focus`, `:active` | PDF has no interaction state. | Style the static state directly. |\n")
	b.WriteString("| Custom HTML elements / Web Components | Folio's HTML parser handles a fixed element set. | Pre-render to a known element (`<div>` / `<span>`) before passing to Folio. |\n")
	b.WriteString("| `position: sticky` | Has no analogue in paginated layout. | Use `@page` running headers/footers via margin boxes. |\n")
	b.WriteString("| ICC profiles for color management | Folio is sRGB-only. | Use sRGB-correct hex values; convert assets to sRGB before embedding. |\n")
	b.WriteString("\n")

	// Final orientation: how to extend the registry.
	b.WriteString("## Adding a new CSS property\n\n")
	b.WriteString("1. Append a `cssProperty` entry to `cssProperties` in `html/css_props.go`.\n")
	b.WriteString("   Required: `Name` and `Apply`. Recommended: `Category`, `Values`, `Notes`.\n")
	b.WriteString("2. Run `go generate ./html/...` to regenerate this document.\n")
	b.WriteString("3. Add at least one row to `TestCSSPropertyParitySnapshot` in\n")
	b.WriteString("   `html/css_props_test.go` asserting the new property's behavior.\n")
	b.WriteString("4. CI guards: `TestCSSDocsInSync` ensures the doc matches the registry,\n")
	b.WriteString("   and `TestNoSwitchRegistryOverlap` ensures no legacy switch case is\n")
	b.WriteString("   reintroduced for a registered property.\n")

	return b.String()
}

//go:generate go run ../internal/gen-css-docs
