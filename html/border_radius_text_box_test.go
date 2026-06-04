// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// Regression tests for issue #329 (part 2): border-radius was dropped when a
// block box directly contained a text run. The block's wrapping Div draws the
// background rounded, but the synthesized child Paragraph re-drew the same
// background as a plain (square) rectangle on top, squaring off the corners.
// The fix clears the redundant block-level background on the child Paragraph
// when the wrapping Div owns the fill.

// findFirstDiv returns the first *layout.Div found by a shallow walk of elems
// (top level only). Returns nil if none.
func findFirstDiv(elems []layout.Element) *layout.Div {
	for _, e := range elems {
		if d, ok := e.(*layout.Div); ok {
			return d
		}
	}
	return nil
}

// firstChildParagraph returns the first direct child *layout.Paragraph of d.
func firstChildParagraph(d *layout.Div) *layout.Paragraph {
	for _, c := range d.Children() {
		if p, ok := c.(*layout.Paragraph); ok {
			return p
		}
	}
	return nil
}

// TestBorderRadiusBoxWithTextChildClearsParagraphBackground reproduces the
// issue: a fixed-radius box containing a direct text run must keep its rounded
// fill on the Div and must NOT re-draw the same fill via the child Paragraph.
func TestBorderRadiusBoxWithTextChildClearsParagraphBackground(t *testing.T) {
	src := `<div style="background:#4F46E5;width:28pt;height:28pt;border-radius:14pt">4</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	div := findFirstDiv(elems)
	if div == nil {
		t.Fatal("expected a wrapping Div carrying the box background")
	}
	if div.Background() == nil {
		t.Fatal("Div should carry the block background")
	}
	r := div.BorderRadii()
	if r[0] == 0 || r[1] == 0 || r[2] == 0 || r[3] == 0 {
		t.Errorf("Div should carry border-radius on all corners, got %v", r)
	}
	p := firstChildParagraph(div)
	if p == nil {
		t.Fatal("expected a child Paragraph for the text run")
	}
	// This is the assertion that FAILS on pre-fix code: the child Paragraph
	// re-drew the same background as a square rectangle.
	if p.Background() != nil {
		t.Errorf("child Paragraph background should be cleared (Div owns the fill), got %+v", *p.Background())
	}
}

// TestBorderRadiusEmptyBoxUnaffected is a regression guard: an empty box with a
// radius still produces a Div with the background and radius (no Paragraph).
func TestBorderRadiusEmptyBoxUnaffected(t *testing.T) {
	src := `<div style="background:#4F46E5;width:28pt;height:28pt;border-radius:14pt"></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	div := findFirstDiv(elems)
	if div == nil {
		t.Fatal("expected a Div for the empty box with visual properties")
	}
	if div.Background() == nil {
		t.Error("Div should carry the block background")
	}
	if div.BorderRadii()[0] == 0 {
		t.Error("Div should carry border-radius")
	}
}

// TestBorderRadiusElementOnlyChildUnaffected is a regression guard: a box whose
// only child is an element (not direct text) keeps Div background + radius, and
// any inner element-level Div keeps its own background.
func TestBorderRadiusElementOnlyChildUnaffected(t *testing.T) {
	src := `<div style="background:#4F46E5;width:28pt;height:28pt;border-radius:14pt"><span>4</span></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	div := findFirstDiv(elems)
	if div == nil {
		t.Fatal("expected a wrapping Div")
	}
	if div.Background() == nil {
		t.Error("Div should carry the block background")
	}
	if div.BorderRadii()[0] == 0 {
		t.Error("Div should carry border-radius")
	}
	// The inline <span> produces an anonymous paragraph child whose
	// block-level background (inherited from the block) must be cleared too.
	if p := firstChildParagraph(div); p != nil && p.Background() != nil {
		t.Errorf("child Paragraph background should be cleared, got %+v", *p.Background())
	}
}

// TestBorderRadiusBoxPreservesInlineSpanHighlight ensures the per-RUN
// BackgroundColor of an inline <span> highlight is NOT cleared by the fix.
// The block-level Paragraph background is cleared, but the run highlight on the
// word "world" must survive.
func TestBorderRadiusBoxPreservesInlineSpanHighlight(t *testing.T) {
	src := `<div style="background:#4F46E5;border-radius:14pt">hello <span style="background:yellow">world</span></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	div := findFirstDiv(elems)
	if div == nil {
		t.Fatal("expected a wrapping Div")
	}
	p := firstChildParagraph(div)
	if p == nil {
		t.Fatal("expected a child Paragraph")
	}
	// Block-level background cleared (Div owns the indigo fill).
	if p.Background() != nil {
		t.Errorf("block-level Paragraph background should be cleared, got %+v", *p.Background())
	}
	// Per-run highlight on "world" must be preserved.
	foundHighlight := false
	for _, run := range p.Runs() {
		if run.BackgroundColor != nil {
			foundHighlight = true
		}
	}
	if !foundHighlight {
		t.Error("inline <span> highlight (per-run BackgroundColor) must be preserved")
	}
}

// findFirstFlex returns the first *layout.Flex found by a shallow walk of elems.
func findFirstFlex(elems []layout.Element) *layout.Flex {
	for _, e := range elems {
		if f, ok := e.(*layout.Flex); ok {
			return f
		}
	}
	return nil
}

// flexItemDiv extracts the *layout.Div carried by a flex item, whether the item
// holds the Div directly or wraps it in a FlexItem.
func flexItemDiv(elem layout.Element) *layout.Div {
	if d, ok := elem.(*layout.Div); ok {
		return d
	}
	return nil
}

// TestBorderRadiusFlexItemWithText covers the flex `flex:0 0 auto` text case
// from the issue: a rounded flex item containing a direct text run inside a real
// `display:flex` parent. Each direct child of the flex container is converted
// via convertNode → convertBlock (the child <div> is display:block), so the same
// fix applies. We assert the flex item's Div keeps its rounded fill while the
// child Paragraph's block-level background is cleared.
func TestBorderRadiusFlexItemWithText(t *testing.T) {
	src := `<div style="display:flex"><div style="flex:0 0 auto;background:#4F46E5;width:28pt;height:28pt;border-radius:14pt">4</div></div>`
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
	div := flexItemDiv(items[0].Element())
	if div == nil {
		t.Fatal("expected the flex item to wrap a rounded Div")
	}
	if div.Background() == nil {
		t.Error("flex item Div should carry the background")
	}
	if div.BorderRadii()[0] == 0 {
		t.Error("flex item Div should carry border-radius")
	}
	if p := firstChildParagraph(div); p != nil && p.Background() != nil {
		t.Errorf("flex item child Paragraph background should be cleared, got %+v", *p.Background())
	}
}

// TestBorderRadiusBoxIndependentParagraphBackgroundKept verifies that a child
// paragraph carrying a DIFFERENT background from the wrapping Div is left alone
// (the clearing is gated on the background matching the Div's fill).
func TestBorderRadiusBoxIndependentParagraphBackgroundKept(t *testing.T) {
	src := `<div style="background:#4F46E5;border-radius:14pt"><p style="background:#FF0000">hi</p></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	div := findFirstDiv(elems)
	if div == nil {
		t.Fatal("expected a wrapping Div")
	}
	// The inner <p style="background:#FF0000"> itself forces its own Div
	// wrapper (needsWrapper), so the outer Div's direct child is that inner
	// Div, not a Paragraph. The outer clearing must not touch it: the outer
	// Div has only Div children, so nothing is cleared. Verify the inner Div
	// still carries its red background.
	var innerDiv *layout.Div
	for _, c := range div.Children() {
		if d, ok := c.(*layout.Div); ok {
			innerDiv = d
		}
	}
	if innerDiv == nil {
		t.Fatal("expected inner Div for the styled <p>")
	}
	if innerDiv.Background() == nil {
		t.Error("inner Div should keep its independent red background")
	}
}

// findFirstTable returns the first *layout.Table found by a shallow walk of
// elems, looking one level into any wrapping Div (a <table> with margins is
// wrapped in a Div). Returns nil if none.
func findFirstTable(elems []layout.Element) *layout.Table {
	for _, e := range elems {
		switch v := e.(type) {
		case *layout.Table:
			return v
		case *layout.Div:
			for _, c := range v.Children() {
				if t, ok := c.(*layout.Table); ok {
					return t
				}
			}
		}
	}
	return nil
}

// firstCell returns the first cell of the first row of a table, or nil.
func firstCell(tbl *layout.Table) *layout.Cell {
	rows := tbl.Rows()
	if len(rows) == 0 {
		return nil
	}
	cells := rows[0].Cells()
	if len(cells) == 0 {
		return nil
	}
	return cells[0]
}

// cellContentParagraph returns the cell's direct content as a *layout.Paragraph,
// either when the content is a Paragraph directly or the first child Paragraph
// of a wrapping Div. Returns nil if no content Paragraph is present.
func cellContentParagraph(cell *layout.Cell) *layout.Paragraph {
	switch v := cell.Content().(type) {
	case *layout.Paragraph:
		return v
	case *layout.Div:
		return firstChildParagraph(v)
	}
	return nil
}

// TestBorderRadiusTableCellWithText reproduces the BLOCKER (B1): a <td> with a
// background + border-radius + direct text must keep the background AND radius
// on the *cell* (which draws the rounded fill), while the cell's content
// Paragraph must NOT carry the same background (which would re-draw a square
// rectangle on top, squaring the corners). Pre-fix the content Paragraph
// carried a non-nil background; this test fails before the B1 fix.
func TestBorderRadiusTableCellWithText(t *testing.T) {
	src := `<table><tr><td style="background:#4F46E5;border-radius:14pt">x</td></tr></table>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	tbl := findFirstTable(elems)
	if tbl == nil {
		t.Fatal("expected a layout.Table")
	}
	cell := firstCell(tbl)
	if cell == nil {
		t.Fatal("expected a cell in the first row")
	}
	if cell.Background() == nil {
		t.Fatal("cell should carry the block background")
	}
	r := cell.BorderRadii()
	if r[0] == 0 || r[1] == 0 || r[2] == 0 || r[3] == 0 {
		t.Errorf("cell should carry border-radius on all corners, got %v", r)
	}
	p := cellContentParagraph(cell)
	if p == nil {
		t.Fatal("expected a content Paragraph for the cell text")
	}
	// The assertion that FAILS on pre-fix code: the content Paragraph re-drew
	// the same background as a square rectangle.
	if p.Background() != nil {
		t.Errorf("cell content Paragraph background should be cleared (cell owns the fill), got %+v", *p.Background())
	}
}

// TestBorderRadiusCSSTableCellWithText is the display:table variant of the
// BLOCKER (B1): a display:table-cell with background + border-radius + direct
// text must keep background + radius on the cell and clear the content
// Paragraph's redundant background.
func TestBorderRadiusCSSTableCellWithText(t *testing.T) {
	src := `<div style="display:table"><div style="display:table-row">` +
		`<div style="display:table-cell;background:#4F46E5;border-radius:14pt">x</div>` +
		`</div></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	tbl := findFirstTable(elems)
	if tbl == nil {
		t.Fatal("expected a layout.Table for display:table")
	}
	cell := firstCell(tbl)
	if cell == nil {
		t.Fatal("expected a cell in the first row")
	}
	if cell.Background() == nil {
		t.Fatal("cell should carry the block background")
	}
	r := cell.BorderRadii()
	if r[0] == 0 || r[1] == 0 || r[2] == 0 || r[3] == 0 {
		t.Errorf("cell should carry border-radius on all corners, got %v", r)
	}
	p := cellContentParagraph(cell)
	if p == nil {
		t.Fatal("expected a content Paragraph for the cell text")
	}
	if p.Background() != nil {
		t.Errorf("cell content Paragraph background should be cleared, got %+v", *p.Background())
	}
}
