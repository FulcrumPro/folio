// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestFlexShorthandWithCalcBasis is an end-to-end regression for two
// connected bugs surfaced by `flex: 0 0 calc(50% - 8px)`:
//
//  1. parseFlexShorthand split the value with strings.Fields, which broke
//     calc(50% - 8px) into 5 tokens; no shorthand case matched, so
//     FlexBasis stayed nil. The flex algorithm fell back to MaxWidth(),
//     producing asymmetric columns sized to intrinsic content.
//  2. cssLengthToUnitValue eagerly resolved any non-percent length
//     (including calc with percentages) against the converter's
//     containerWidth — which does not account for page margins. So
//     calc(50% - 8px) of 541.28pt was 264.64pt instead of
//     calc(50% - 8px) of the actual layout area 456.24pt.
//
// Symptom in the contact-profile reduction case: a `flex: 1` value
// sibling collapsed to one character per line because its column ended
// up narrower than the sibling `width: 170px; flex-shrink: 0` term.
//
// Geometry chosen so all numbers are stable regardless of future
// default-margin changes — the test pins the layout area explicitly via
// PlanLayout rather than relying on document margins.
func TestFlexShorthandWithCalcBasis(t *testing.T) {
	htmlDoc := `<!DOCTYPE html><html><head><style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
.pdf-page { padding: 28px 36px; font-size: 13px; }
.grid-2 { display: flex; flex-wrap: wrap; gap: 16px; }
.grid-2 > * { flex: 0 0 calc(50% - 8px); min-width: 0; }
.def-row { display: flex; gap: 16px; }
.def-term { width: 170px; flex-shrink: 0; }
.def-value { flex: 1; }
</style></head><body><div class="pdf-page">
<div class="grid-2">
  <div><div class="def-row"><div class="def-term">Email</div><div class="def-value">brian.halligan@hubspot.com</div></div></div>
  <div><div class="def-row"><div class="def-term">HubSpot</div><div class="def-value">Software</div></div></div>
</div>
</div></body></html>`

	// Pin the layout area explicitly so the test does not drift if the
	// document's default margins change.
	const pageW = 595.28
	const layoutArea = 510.24 // = pageW - 42.52*2

	elems, err := Convert(htmlDoc, &Options{PageWidth: pageW, PageHeight: 841.89})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(elems) != 1 {
		t.Fatalf("got %d top-level elements, want 1", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: layoutArea, Height: 1e9})

	// .pdf-page has padding 36px = 27pt L/R; inner = 510.24 - 54 = 456.24pt.
	// Each grid-2 column is calc(50% - 8px) = 50% × 456.24 - 6 = 222.12pt.
	// Two columns + 12pt gap (16px) should sum back to 456.24pt exactly.
	const wantCol = 222.12
	const wantGap = 12.0 // 16px = 12pt

	cols := findGridColumns(plan.Blocks)
	if len(cols) != 2 {
		t.Fatalf("expected 2 grid-2 columns at the same Y, got %d", len(cols))
	}

	// Each column is the expected calc-resolved width.
	for i, c := range cols {
		if math.Abs(c.Width-wantCol) > 0.5 {
			t.Errorf("column %d width = %.2f, want ~%.2f", i, c.Width, wantCol)
		}
	}

	// Side-by-side, not stacked: cols[0] at x=0, cols[1] at x=cols[0].Width+gap.
	if cols[0].X != 0 {
		t.Errorf("cols[0].X = %.2f, want 0", cols[0].X)
	}
	wantSecondX := cols[0].Width + wantGap
	if math.Abs(cols[1].X-wantSecondX) > 0.5 {
		t.Errorf("cols[1].X = %.2f, want ~%.2f (cols[0].Width + gap)", cols[1].X, wantSecondX)
	}

	// Both cols at the same Y rules out vertical stacking — which is what
	// happened when calc resolved against the wrong (too-large) container
	// and items wrapped to separate lines.
	if cols[0].Y != cols[1].Y {
		t.Errorf("cols Y differ: %.2f vs %.2f — items stacked vertically", cols[0].Y, cols[1].Y)
	}
}

// findGridColumns walks the layout tree and returns the two grid columns:
// the first parent block whose direct children include exactly two boxes
// at Y == 0 with positive width and distinct X positions. The repro
// document is shallow and unique enough that this match is unambiguous;
// tests assert the X/gap geometry afterwards to catch any false positives.
func findGridColumns(blocks []layout.PlacedBlock) []layout.PlacedBlock {
	for _, b := range blocks {
		var atTop []layout.PlacedBlock
		for _, c := range b.Children {
			if c.Y == 0 && c.Width > 0 {
				atTop = append(atTop, c)
			}
		}
		if len(atTop) == 2 && atTop[0].X != atTop[1].X {
			return atTop
		}
		if cs := findGridColumns(b.Children); len(cs) == 2 {
			return cs
		}
	}
	return nil
}
