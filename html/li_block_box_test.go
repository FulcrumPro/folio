// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/layout"
)

// distinctContentYs returns the number of distinct y-positions of leaf content
// blocks in a plan, used to count rendered text lines.
func distinctContentYs(plan layout.LayoutPlan) int {
	ys := map[float64]bool{}
	var walk func(bs []layout.PlacedBlock, off float64)
	walk = func(bs []layout.PlacedBlock, off float64) {
		for _, b := range bs {
			if len(b.Children) > 0 {
				walk(b.Children, off+b.Y)
				continue
			}
			if b.Height > 0 {
				ys[off+b.Y] = true
			}
		}
	}
	walk(plan.Blocks, 0)
	return len(ys)
}

// drawPlanToStream draws every block (and its children) of a plan into a fresh
// content stream and returns the operators as a string.
func drawPlanToStream(plan layout.LayoutPlan) string {
	page := &layout.PageResult{Stream: content.NewStream()}
	ctx := layout.DrawContext{Stream: page.Stream, Page: page}
	var draw func(bs []layout.PlacedBlock, topY float64)
	draw = func(bs []layout.PlacedBlock, topY float64) {
		for _, b := range bs {
			if b.Draw != nil {
				b.Draw(ctx, b.X, topY-b.Y)
			}
			if len(b.Children) > 0 {
				draw(b.Children, topY-b.Y)
			}
		}
	}
	draw(plan.Blocks, 1000)
	return string(page.Stream.Bytes())
}

// TestLiBlockChildrenProduceMultipleLines is the structural regression for
// #342: an <li> with block-level children must lay out on multiple lines, not
// flattened onto one.
func TestLiBlockChildrenProduceMultipleLines(t *testing.T) {
	htmlStr := `<ol><li><u>Title.</u><div>First body.</div><div>Second body.</div></li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if n := distinctContentYs(plan); n < 3 {
		t.Errorf("expected >=3 distinct content lines for block <li> children, got %d", n)
	}
}

// TestLiBlockChildrenMatchDivEquivalent confirms that the same block markup
// inside an <li> produces the same number of lines as inside a <div>.
func TestLiBlockChildrenMatchDivEquivalent(t *testing.T) {
	inner := `<u>Title.</u><div>First body.</div><div>Second body.</div>`

	liElems, err := Convert(`<ol><li>`+inner+`</li></ol>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Wrap the div equivalent in an outer box so Convert keeps it as a single
	// element (bare top-level block children are hoisted to siblings).
	divElems, err := Convert(`<div style="padding:0.1px">`+inner+`</div>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	liLines := distinctContentYs(liElems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000}))
	divLines := distinctContentYs(divElems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000}))
	if liLines != divLines {
		t.Errorf("li produced %d lines, div produced %d; expected equal", liLines, divLines)
	}
}

// TestLiStyledBoxRendersRoundedFill is the structural regression for #339: a
// styled <li> (background + border-radius + padding) must paint a rounded
// filled box.
func TestLiStyledBoxRendersRoundedFill(t *testing.T) {
	htmlStr := `<ul><li style="background:#4F46E5;color:#fff;border-radius:8px;padding:6px 10px">item</li></ul>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	ops := drawPlanToStream(plan)

	// Background fill color #4F46E5 => 0.309804 0.27451 0.898039 rg.
	if !strings.Contains(ops, "0.309804 0.27451 0.898039 rg") {
		t.Errorf("expected indigo background fill in content stream, got:\n%s", ops)
	}
	// Rounded corners are emitted as Bézier curves (`c` operator).
	if !strings.Contains(ops, " c\n") && !strings.Contains(ops, " c ") {
		t.Errorf("expected Bézier curves for rounded corners, got:\n%s", ops)
	}
	// A fill operator must follow the box path.
	if !strings.Contains(ops, "\nf\n") && !strings.HasSuffix(strings.TrimSpace(ops), "f") &&
		!strings.Contains(ops, " f\n") {
		t.Errorf("expected fill operator for the box, got:\n%s", ops)
	}
}

// TestLiPlainInlineUsesRunsPath is a regression guard: a plain inline <li>
// must keep the existing runs path (marker inline with text on one line) and
// not be wrapped in an element/Div.
func TestLiPlainInlineUsesRunsPath(t *testing.T) {
	htmlStr := `<ul><li>Just text</li><li>Another</li></ul>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	// Two single-line items => two content lines.
	if n := distinctContentYs(plan); n != 2 {
		t.Errorf("expected 2 content lines for two plain inline items, got %d", n)
	}
	// The inline runs path draws marker + text in the same LI line block; no
	// Bézier curve fill should appear (no box).
	ops := drawPlanToStream(plan)
	if strings.Contains(ops, " c\n") {
		t.Errorf("plain inline <li> should not emit a rounded box, got:\n%s", ops)
	}
}

// TestLiNestedListStillRenders verifies a nested list inside an <li> renders
// with its own marker and deeper indentation (sub-list fast path preserved).
func TestLiNestedListStillRenders(t *testing.T) {
	htmlStr := `<ul><li>Parent<ul><li>Child</li></ul></li></ul>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	ops := drawPlanToStream(plan)

	// The runs path applies item indentation at draw time via the text Td
	// x-offset. Both the parent text and the deeper child text must appear,
	// with the child drawn at a larger x-offset than the parent.
	parentX, parentOK := textTdX(ops, "arent")
	childX, childOK := textTdX(ops, "Child")
	if !parentOK || !childOK {
		t.Fatalf("expected both parent and child text drawn; parent=%v child=%v\n%s", parentOK, childOK, ops)
	}
	if childX <= parentX {
		t.Errorf("expected nested child more indented than parent (parentX=%f childX=%f)", parentX, childX)
	}
}

// textTdX scans content-stream operators for a Td positioning command
// immediately preceding a text-show operator (Tj/TJ) whose payload contains
// the given substring, returning the Td x-coordinate.
func textTdX(ops, substr string) (float64, bool) {
	lines := strings.Split(ops, "\n")
	lastX := 0.0
	haveX := false
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasSuffix(l, " Td") {
			var x, y float64
			if _, err := fmt.Sscanf(l, "%f %f Td", &x, &y); err == nil {
				lastX = x
				haveX = true
			}
		}
		if (strings.Contains(l, "Tj") || strings.Contains(l, "TJ")) &&
			strings.Contains(l, substr) && haveX {
			return lastX, true
		}
	}
	return 0, false
}

// firstLeafX returns the X of the first (topmost) leaf content block in a
// subtree, relative to the root of the supplied slice (X accumulated).
func firstLeafX(blocks []layout.PlacedBlock) (float64, bool) {
	bestSet := false
	bestY := 0.0
	bestX := 0.0
	var walk func(bs []layout.PlacedBlock, offX, offY float64)
	walk = func(bs []layout.PlacedBlock, offX, offY float64) {
		for _, b := range bs {
			if len(b.Children) > 0 {
				walk(b.Children, offX+b.X, offY+b.Y)
				continue
			}
			if b.Height <= 0 {
				continue
			}
			// Skip the marker block (leaf "LI" at X==0 with width == indent).
			if b.Tag == "LI" && offX+b.X == 0 && b.Width == 18 {
				continue
			}
			if !bestSet || offY+b.Y < bestY {
				bestSet = true
				bestY = offY + b.Y
				bestX = offX + b.X
			}
		}
	}
	walk(blocks, 0, 0)
	return bestX, bestSet
}

// TestLiPercentPaddingResolvesAgainstContentColumn verifies the SHOULD-FIX:
// percentage padding on a styled <li> resolves against the content column
// (page width minus the list indent), not the full page width. With a
// PageWidth of 618 and the default list indent of 18, the content column is
// 600pt, so padding:10% must be 60pt — not 61.8pt (10% of the full page).
func TestLiPercentPaddingResolvesAgainstContentColumn(t *testing.T) {
	const pageW = 618.0
	htmlStr := `<ul><li style="padding:10%;background:#eee">item</li></ul>`
	elems, err := Convert(htmlStr, &Options{PageWidth: pageW})
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: pageW, Height: 1000})

	// The first text leaf sits inset from the left edge by (list indent +
	// padding-left). The list draws the element column at indent=18, and the
	// Div insets its content by padding-left. So leafX = indent + paddingLeft.
	const indent = 18.0
	leafX, ok := firstLeafX(plan.Blocks)
	if !ok {
		t.Fatal("no content leaf found")
	}
	paddingLeft := leafX - indent

	contentCol := pageW - indent // 600
	wantPad := 0.10 * contentCol // 60
	wrongPad := 0.10 * pageW     // 61.8

	if diff := paddingLeft - wantPad; diff < -1 || diff > 1 {
		t.Errorf("padding-left = %.3f, want ≈%.3f (10%% of content column %.0f); got closer to %.3f (10%% of full page)",
			paddingLeft, wantPad, contentCol, wrongPad)
	}

	// The styled box must not exceed the content column. The Div container
	// block width should be <= contentCol.
	var maxRight float64
	var walk func(bs []layout.PlacedBlock, offX float64)
	walk = func(bs []layout.PlacedBlock, offX float64) {
		for _, b := range bs {
			if r := offX + b.X + b.Width; r > maxRight {
				maxRight = r
			}
			walk(b.Children, offX+b.X)
		}
	}
	walk(plan.Blocks, 0)
	if maxRight > pageW+0.5 {
		t.Errorf("box right edge %.3f exceeds page width %.0f", maxRight, pageW)
	}
}

// TestLiStyledBoxIsElementItem confirms a styled <li> goes through the element
// path (single multi-line-capable item) rather than the inline runs path.
func TestLiStyledBoxIsElementItem(t *testing.T) {
	htmlStr := `<ul><li style="background:#eee;border-radius:6px;padding:4px 8px">item</li></ul>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	ops := drawPlanToStream(plan)
	// #eee => 0.933333 grey fill.
	if !strings.Contains(ops, "0.933333 0.933333 0.933333 rg") {
		t.Errorf("expected #eee background fill, got:\n%s", ops)
	}
	if !strings.Contains(ops, " c\n") && !strings.Contains(ops, " c ") {
		t.Errorf("expected rounded-corner Bézier curves, got:\n%s", ops)
	}
}
