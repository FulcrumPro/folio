// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestRowFlexHonorsItemMargins pins the v0.9.1-fulcrum.19 patch: horizontal
// margins on a row flex item must occupy main-axis space (and survive
// flex-shrink), so inter-item gaps don't collapse when the row overflows.
//
// folio's row flex ignored item horizontal margins entirely: resolveGrowShrink
// didn't reserve them and computeJustifyOffsets positioned items by content
// width alone. When the row fit there was incidental spacing, but when it
// overflowed, shrink swallowed the gap. The v3 commerce header
// (`.mirrored { display:flex }`, a company-info column with `margin-left` + an
// `<hr>` separator) collapsed to a ~0.7pt gap with the embedded Arial-metric
// font, so the two contact columns abutted ("Created ByAcme Industries");
// Chrome keeps an ~8pt gap.
//
// The fix reserves item horizontal margins in resolveGrowShrink (CSS Flexbox
// §9.7: margins never flex) and shifts each item by its margin-left in
// placement.
func TestRowFlexHonorsItemMargins(t *testing.T) {
	render := func(margin string) float64 {
		src := `<html><head><style>
			body{margin:0;padding:0}
			.row{display:flex;width:100pt}
			.a{width:70pt;flex-shrink:0}
			.b{width:70pt;flex-shrink:0;margin-left:` + margin + `}
		</style></head><body>
			<div class="row"><div class="a">AAA</div><div class="b">BBB</div></div>
		</body></html>`
		pdf, err := renderHTMLToPDF(src)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return findTextX(pdf, "BBB")
	}
	// The row overflows (70+70 > 100), so the items shrink. The second item's
	// margin-left must still push it right — independent of, and on top of, the
	// shrink. Compare a 20pt margin against no margin: the gap delta should be
	// close to 20pt. Pre-patch the margin was ignored, so both were identical.
	noMargin := render("0")
	withMargin := render("20pt")
	t.Logf("BBB.x: no-margin=%.1f  margin-left:20pt=%.1f  delta=%.1f", noMargin, withMargin, withMargin-noMargin)
	if noMargin < 0 || withMargin < 0 {
		t.Fatalf("BBB not found: no-margin=%.1f with-margin=%.1f", noMargin, withMargin)
	}
	if d := withMargin - noMargin; d < 15 || d > 25 {
		t.Errorf("margin-left:20pt shifted the item by %.1fpt, expected ~20 — row-flex item margin not honored", d)
	}
}
