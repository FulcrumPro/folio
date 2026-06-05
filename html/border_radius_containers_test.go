// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// Regression tests for issue #329 (containers batch): a container that should
// draw a rounded (border-radius) background failed to when it contained text —
// either the radius was dropped entirely (inline element blockified as a
// flex/grid item, or display:block; hand-built blockquote/figure/table wrapper
// Divs) or a child Paragraph repainted a square background over the rounded
// fill. These tests assert structurally that the rounded fill is present and
// not over-painted.

// findFirstGrid returns the first *layout.Grid found by a shallow walk of elems.
func findFirstGrid(elems []layout.Element) *layout.Grid {
	for _, e := range elems {
		if g, ok := e.(*layout.Grid); ok {
			return g
		}
	}
	return nil
}

// nonZeroRadius reports whether a Div carries any border-radius, absolute or
// percentage, on any corner.
func nonZeroRadius(d *layout.Div) bool {
	r := d.BorderRadii()
	if r[0] != 0 || r[1] != 0 || r[2] != 0 || r[3] != 0 {
		return true
	}
	p := d.BorderRadiusPercent()
	return p[0] != 0 || p[1] != 0 || p[2] != 0 || p[3] != 0
}

// --- Mechanism A: inline element with a box, blockified as flex/grid item ---

// flexSpanRepro builds a display:flex parent with a single rounded <span>
// flex item carrying the supplied flex shorthand.
func flexSpanRepro(flexShorthand string) string {
	return `<div style="display:flex"><span style="` + flexShorthand +
		`background:#4F46E5;color:#fff;border-radius:10pt;padding:6pt 14pt">CHIP</span></div>`
}

// assertRoundedFlexItem asserts the flex container's single item is a Div with a
// non-zero border-radius and a background, and that any inner Paragraph has its
// background cleared (no square overdraw).
func assertRoundedFlexItem(t *testing.T, src string) {
	t.Helper()
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	flex := findFirstFlex(elems)
	if flex == nil {
		t.Fatal("expected a layout.Flex for the display:flex parent")
	}
	items := flex.Items()
	if len(items) != 1 {
		t.Fatalf("expected exactly one flex item, got %d", len(items))
	}
	div, ok := items[0].Element().(*layout.Div)
	if !ok {
		t.Fatalf("expected the flex item to be a *layout.Div owning the rounded fill, got %T", items[0].Element())
	}
	if div.Background() == nil {
		t.Error("flex item Div should carry the background")
	}
	if !nonZeroRadius(div) {
		t.Errorf("flex item Div should carry border-radius, got %v / %v", div.BorderRadii(), div.BorderRadiusPercent())
	}
	if p := firstChildParagraph(div); p != nil && p.Background() != nil {
		t.Errorf("flex item child Paragraph background should be cleared (Div owns the fill), got %+v", *p.Background())
	}
}

// TestBorderRadiusFlexSpanFlexNone covers the euskadi31 repro: a rounded
// <span> with flex:0 0 auto inside a display:flex parent must render as a
// rounded Div, not a square Paragraph.
func TestBorderRadiusFlexSpanFlexNone(t *testing.T) {
	assertRoundedFlexItem(t, flexSpanRepro("flex:0 0 auto;"))
}

// chipDivPadding converts src, expecting either a flex item, a grid child, or a
// top-level Div, and returns the chip's wrapping Div padding. It fails if no
// such Div is found.
func chipDivPadding(t *testing.T, src string) layout.Padding {
	t.Helper()
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if flex := findFirstFlex(elems); flex != nil && len(flex.Items()) == 1 {
		if div, ok := flex.Items()[0].Element().(*layout.Div); ok {
			return div.Padding()
		}
	}
	if grid := findFirstGrid(elems); grid != nil && len(grid.Children()) == 1 {
		if div, ok := grid.Children()[0].(*layout.Div); ok {
			return div.Padding()
		}
	}
	// A top-level inline-block span flows inline and is buffered into a
	// Paragraph as an atomic inline box (TextRun.InlineElement).
	for _, e := range elems {
		if p, ok := e.(*layout.Paragraph); ok {
			for _, r := range p.Runs() {
				if div, ok := r.InlineElement.(*layout.Div); ok {
					return div.Padding()
				}
			}
		}
	}
	if div := findFirstDiv(elems); div != nil {
		return div.Padding()
	}
	t.Fatalf("no chip Div found for src %q", src)
	return layout.Padding{}
}

// TestBorderRadiusFlexSpanChipPaddingMatchesInlineBlock is the issue #340 BUG 1
// fix: a blockified <span> chip (flex item or grid item) carrying a rounded box
// must render its full box model — notably equal left/right padding — IDENTICAL
// to the display:inline-block chip, instead of dropping the padding. Earlier the
// flex/grid path applied only background + radius, leaving the text flush against
// the left edge (0pt left padding) while inline-block applied 12pt both sides.
func TestBorderRadiusFlexSpanChipPaddingMatchesInlineBlock(t *testing.T) {
	const box = `background:#4F46E5;color:#fff;border-radius:8pt;padding:5pt 12pt`

	inlineBlock := chipDivPadding(t,
		`<span style="display:inline-block;`+box+`">CHIP</span>`)
	// Sanity: the reference inline-block chip has the expected 12/5 padding.
	if inlineBlock.Left != 12 || inlineBlock.Right != 12 || inlineBlock.Top != 5 || inlineBlock.Bottom != 5 {
		t.Fatalf("inline-block reference padding unexpected: %+v", inlineBlock)
	}

	flex := chipDivPadding(t,
		`<div style="display:flex"><span style="flex:0 0 auto;`+box+`">CHIP</span></div>`)
	if flex != inlineBlock {
		t.Errorf("flex chip padding %+v != inline-block %+v", flex, inlineBlock)
	}
	if flex.Left != flex.Right {
		t.Errorf("flex chip left/right padding must be equal, got L=%v R=%v", flex.Left, flex.Right)
	}

	grid := chipDivPadding(t,
		`<div style="display:grid;grid-template-columns:auto"><span style="`+box+`">CHIP</span></div>`)
	if grid != inlineBlock {
		t.Errorf("grid chip padding %+v != inline-block %+v", grid, inlineBlock)
	}
	if grid.Left != grid.Right {
		t.Errorf("grid chip left/right padding must be equal, got L=%v R=%v", grid.Left, grid.Right)
	}
}

func TestBorderRadiusFlexSpanPercentRadius(t *testing.T) {
	// border-radius:50% (the #332 elliptical/percentage path) must reach the
	// blockified flex-item Div as a percentage fraction, not be dropped.
	src := `<div style="display:flex"><span style="flex:0 0 auto;` +
		`background:#4F46E5;color:#fff;border-radius:50%;padding:6pt 14pt">CHIP</span></div>`
	assertRoundedFlexItem(t, src)

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	flex := findFirstFlex(elems)
	if flex == nil || len(flex.Items()) != 1 {
		t.Fatal("expected one flex item")
	}
	div, ok := flex.Items()[0].Element().(*layout.Div)
	if !ok {
		t.Fatalf("expected flex item *layout.Div, got %T", flex.Items()[0].Element())
	}
	if pct := div.BorderRadiusPercent(); pct[0] == 0 {
		t.Errorf("border-radius:50%% should reach the flex item Div as a percentage, got %v", pct)
	}
}

// TestBorderRadiusFlexSpanFlexGrow covers a rounded <span> with flex:1.
func TestBorderRadiusFlexSpanFlexGrow(t *testing.T) {
	assertRoundedFlexItem(t, flexSpanRepro("flex:1;"))
}

// TestBorderRadiusFlexSpanDefault covers a rounded <span> with no flex
// shorthand (default flex behavior) inside a display:flex parent.
func TestBorderRadiusFlexSpanDefault(t *testing.T) {
	assertRoundedFlexItem(t, flexSpanRepro(""))
}

// TestBorderRadiusGridSpan covers a rounded <span> grid item inside a
// display:grid parent: it must render as a rounded Div, not a square Paragraph.
func TestBorderRadiusGridSpan(t *testing.T) {
	src := `<div style="display:grid;grid-template-columns:1fr">` +
		`<span style="background:#4F46E5;color:#fff;border-radius:10pt;padding:6pt 14pt">CHIP</span>` +
		`</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	grid := findFirstGrid(elems)
	if grid == nil {
		t.Fatal("expected a layout.Grid for the display:grid parent")
	}
	children := grid.Children()
	if len(children) != 1 {
		t.Fatalf("expected exactly one grid child, got %d", len(children))
	}
	div, ok := children[0].(*layout.Div)
	if !ok {
		t.Fatalf("expected the grid child to be a *layout.Div owning the rounded fill, got %T", children[0])
	}
	if div.Background() == nil {
		t.Error("grid child Div should carry the background")
	}
	if !nonZeroRadius(div) {
		t.Errorf("grid child Div should carry border-radius, got %v / %v", div.BorderRadii(), div.BorderRadiusPercent())
	}
	if p := firstChildParagraph(div); p != nil && p.Background() != nil {
		t.Errorf("grid child Paragraph background should be cleared, got %+v", *p.Background())
	}
}

// TestBorderRadiusDisplayBlockSpan covers a default-inline <span> set to
// display:block carrying background + border-radius: it must render as a
// rounded Div with the child Paragraph background cleared.
func TestBorderRadiusDisplayBlockSpan(t *testing.T) {
	src := `<span style="display:block;background:#4F46E5;color:#fff;border-radius:10pt;padding:6pt 14pt">CHIP</span>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	div := findFirstDiv(elems)
	if div == nil {
		t.Fatal("expected a wrapping Div for the display:block span")
	}
	if div.Background() == nil {
		t.Error("Div should carry the background")
	}
	if !nonZeroRadius(div) {
		t.Errorf("Div should carry border-radius, got %v / %v", div.BorderRadii(), div.BorderRadiusPercent())
	}
	if p := firstChildParagraph(div); p != nil && p.Background() != nil {
		t.Errorf("child Paragraph background should be cleared, got %+v", *p.Background())
	}
}

// TestBorderRadiusFlexSpanNoBoxStaysInline is the REGRESSION GUARD: a <span>
// WITHOUT a box (no border-radius, no background) inside a flex container must
// stay a bare inline Paragraph — it must NOT be wrapped in a Div.
func TestBorderRadiusFlexSpanNoBoxStaysInline(t *testing.T) {
	src := `<div style="display:flex"><span style="color:#333">PLAIN</span></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	flex := findFirstFlex(elems)
	if flex == nil {
		t.Fatal("expected a layout.Flex")
	}
	items := flex.Items()
	if len(items) != 1 {
		t.Fatalf("expected exactly one flex item, got %d", len(items))
	}
	if _, ok := items[0].Element().(*layout.Paragraph); !ok {
		t.Fatalf("a no-box span must stay a bare inline Paragraph, got %T", items[0].Element())
	}
}

// TestBorderRadiusFlexSpanBackgroundNoRadiusStaysInline is a second regression
// guard: a <span> with a background but NO border-radius must not be wrapped,
// since there is no rounded fill to own. It stays a bare Paragraph (which still
// paints its own square background, matching prior behavior).
func TestBorderRadiusFlexSpanBackgroundNoRadiusStaysInline(t *testing.T) {
	src := `<div style="display:flex"><span style="background:#4F46E5;color:#fff">CHIP</span></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	flex := findFirstFlex(elems)
	if flex == nil {
		t.Fatal("expected a layout.Flex")
	}
	items := flex.Items()
	if len(items) != 1 {
		t.Fatalf("expected exactly one flex item, got %d", len(items))
	}
	if _, ok := items[0].Element().(*layout.Paragraph); !ok {
		t.Fatalf("a span with background but no radius must stay a bare Paragraph, got %T", items[0].Element())
	}
}

// --- Mechanism B: hand-built wrapper Divs ---

// TestBorderRadiusBlockquote covers <blockquote> with background + border-radius
// + text: the wrapper Div must carry the radius AND its child Paragraph
// background must be cleared (no square overpaint).
func TestBorderRadiusBlockquote(t *testing.T) {
	src := `<blockquote style="background:#4F46E5;color:#fff;border-radius:10pt">quoted</blockquote>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	div := findFirstDiv(elems)
	if div == nil {
		t.Fatal("expected a wrapping Div for the blockquote")
	}
	if div.Background() == nil {
		t.Error("blockquote Div should carry the background")
	}
	if !nonZeroRadius(div) {
		t.Errorf("blockquote Div should carry border-radius, got %v / %v", div.BorderRadii(), div.BorderRadiusPercent())
	}
	p := firstChildParagraph(div)
	if p == nil {
		t.Fatal("expected a child Paragraph for the blockquote text")
	}
	if p.Background() != nil {
		t.Errorf("blockquote child Paragraph background should be cleared (Div owns the fill), got %+v", *p.Background())
	}
}

// TestBlockquoteAccentRoundedInnerFill renders a rounded blockquote and a plain
// blockquote through the full pipeline and asserts the default gray left accent
// renders as an INNER filled stripe clipped to the rounded shape (filled rect
// after a clip, no stroked accent) on the rounded one, while the plain one keeps
// its straight stroked accent (issue #329).
func TestBlockquoteAccentRoundedInnerFill(t *testing.T) {
	// Rounded blockquote: accent must be a clipped filled stripe, no stroke.
	roundedSrc := `<blockquote style="background:#FEF3C7;border-radius:10pt;padding:8pt 12pt">quoted text in a rounded block</blockquote>`
	roundedElems, err := Convert(roundedSrc, nil)
	if err != nil {
		t.Fatal(err)
	}
	margins := layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72}
	r := layout.NewRenderer(612, 792, margins)
	for _, e := range roundedElems {
		r.Add(e)
	}
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page (rounded)")
	}
	rounded := string(pages[0].Stream.Bytes())

	// The gray accent fill (0.6 0.6 0.6 rg) must be present as a filled stripe,
	// and there must be NO stroked gray accent (0.6 0.6 0.6 RG) for the rounded
	// case — the accent is an inner clipped fill, not an outline stroke.
	if !strings.Contains(rounded, "0.6 0.6 0.6 rg") {
		t.Errorf("rounded blockquote: expected gray accent FILL (0.6 0.6 0.6 rg); stream:\n%s", rounded)
	}
	if strings.Contains(rounded, "0.6 0.6 0.6 RG") {
		t.Errorf("rounded blockquote: must NOT stroke the gray accent (0.6 0.6 0.6 RG); stream:\n%s", rounded)
	}

	// Plain blockquote (no radius): accent must still be a straight stroked bar.
	plainSrc := `<blockquote>plain quote with the default straight accent</blockquote>`
	plainElems, err := Convert(plainSrc, nil)
	if err != nil {
		t.Fatal(err)
	}
	r2 := layout.NewRenderer(612, 792, margins)
	for _, e := range plainElems {
		r2.Add(e)
	}
	pages2 := r2.Render()
	if len(pages2) == 0 {
		t.Fatal("expected at least 1 page (plain)")
	}
	plain := string(pages2[0].Stream.Bytes())
	if !strings.Contains(plain, "0.6 0.6 0.6 RG") {
		t.Errorf("plain blockquote: expected straight stroked gray accent (0.6 0.6 0.6 RG); stream:\n%s", plain)
	}
}

// TestBorderRadiusFigure covers <figure> with background + border-radius: the
// wrapper Div must carry the radius.
func TestBorderRadiusFigure(t *testing.T) {
	src := `<figure style="background:#4F46E5;border-radius:10pt"><figcaption>cap</figcaption></figure>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	div := findFirstDiv(elems)
	if div == nil {
		t.Fatal("expected a wrapping Div for the figure")
	}
	if div.Background() == nil {
		t.Error("figure Div should carry the background")
	}
	if !nonZeroRadius(div) {
		t.Errorf("figure Div should carry border-radius, got %v / %v", div.BorderRadii(), div.BorderRadiusPercent())
	}
}

// TestBorderRadiusTableWrapper covers the whole-table wrapper: a <table> with
// background + border-radius must produce a wrapper Div that carries the radius.
func TestBorderRadiusTableWrapper(t *testing.T) {
	src := `<table style="background:#4F46E5;border-radius:10pt"><tr><td>x</td></tr></table>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	div := findFirstDiv(elems)
	if div == nil {
		t.Fatal("expected a wrapper Div for the table")
	}
	if div.Background() == nil {
		t.Error("table wrapper Div should carry the background")
	}
	if !nonZeroRadius(div) {
		t.Errorf("table wrapper Div should carry border-radius, got %v / %v", div.BorderRadii(), div.BorderRadiusPercent())
	}
	// Sanity: the wrapper holds the Table.
	if findFirstTable(elems) == nil {
		t.Error("expected the wrapper to contain a layout.Table")
	}
}
