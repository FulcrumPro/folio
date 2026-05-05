package fulcrum

import (
	"testing"
)

// TestBottomAnchoredAbsolutePosition pins the v0.7.1-fulcrum.8 patch:
// CSS `position: fixed; bottom: 0` (and the absolute equivalent) must
// place the BOTTOM edge of the element at the requested Y, not the
// TOP. Before this patch, an inline footer with `bottom: 0` was
// drawn with its TOP at page Y=0 — placing the entire element below
// the page bottom and making it invisible.
//
// Symptom in production: the .NET v3 sales/purchasing/shipping
// templates declare a `<div id="pageFooter">` that becomes
// `position: fixed; bottom: 0` inside `@media print`. The footer
// holds "Last Modified by …" and "Powered By // fulcrum"; in folio
// it disappeared entirely because drawBlock placed it outside the
// rendered page area.
//
// Patch: thread a BottomAnchored flag through the html-converter →
// tmpl → document → layout chain. When set, the renderer adds the
// element's measured height to the requested Y before drawing so
// the bottom edge lands at Y instead of the top.
//
// We observe the fix by rendering a `<div style="position:fixed;
// bottom:0">FOOTER</div>` and asserting its rendered Y is at or
// near the page bottom (between 0 and 30pt) rather than negative.
func TestBottomAnchoredAbsolutePosition(t *testing.T) {
	src := `<html><head><style>
		.foot { position: fixed; bottom: 0; }
	</style></head><body>
		<p>main content</p>
		<div class="foot">FOOTER</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	y := findTextY(pdf, "FOOTER")
	if y < 0 {
		t.Fatalf("'FOOTER' not found in rendered PDF — element was dropped from output")
	}
	// PDF Y is bottom-up; bottom: 0 should place the element near Y=0
	// (positive but small, accounting for the font baseline within the
	// element's box). Pre-patch, the element drew at Y≈-10pt. Allow
	// some slack on the upper bound for font metrics + padding while
	// catching anything that drifts back into negative or off-page.
	if y < 0 || y > 30 {
		t.Errorf("'FOOTER' rendered at Y=%.1fpt; expected 0–30pt for bottom-anchored positioning. Negative Y means the element is below the page bottom (the bug we're fixing); a high Y means bottom-anchoring isn't being applied.",
			y)
	}
}
