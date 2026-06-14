// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestGridRowWithPercentHeightChildRenders pins the v0.9.1-fulcrum.28 patch: a
// `height: 100%` child inside a grid cell must not break the grid's row sizing.
//
// A percentage height resolves to `auto` when the containing block's height is
// indefinite (CSS 2.1 §10.5) — which it always is in folio's flow layout.
// folio instead resolved it against the remaining page height, blowing the
// child up to ~page height and collapsing/dropping the grid rows. The .NET
// DocGen ExtremeShipping packing slip lost its entire PRODUCT list: each
// line-item row holds a `height: 100%` barcode-centering div.
func TestGridRowWithPercentHeightChildRenders(t *testing.T) {
	mk := func(style string) string {
		return `<html><head><style>body{margin:0;padding:0;font-size:10pt}
			.grid{display:grid;grid-template-columns:50% 50%}
		</style></head><body>
			<div class="grid"><div>ROWONE-A</div><div>ROWONE-B</div></div>
			<div class="grid"><div><div style="` + style + `">ROWTWO-A</div></div><div>ROWTWO-B</div></div>
			<div class="grid"><div>ROWTHREE-A</div><div>ROWTHREE-B</div></div>
		</body></html>`
	}
	ys := func(style string) (r1, r2, r3 float64) {
		pdf, err := renderHTMLToPDF(mk(style))
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return findTextY(pdf, "ROWONE-A"), findTextY(pdf, "ROWTWO-A"), findTextY(pdf, "ROWTHREE-A")
	}
	// Baseline: rows stack top-to-bottom (PDF y is top-origin-inverted, so each
	// row has a strictly smaller y than the one above).
	b1, b2, b3 := ys("")
	// With a height:100% child, the rows must still stack the same way — both
	// for a plain block child (block path) and a display:flex child (flex path;
	// the ExtremeShipping barcode div is display:flex, the case fulcrum.28
	// missed and fulcrum.29 fixed).
	for _, style := range []string{"height:100%", "display:flex;align-items:center;height:100%"} {
		h1, h2, h3 := ys(style)
		t.Logf("baseline y: %.0f %.0f %.0f ; [%s] y: %.0f %.0f %.0f", b1, b2, b3, style, h1, h2, h3)
		if !(h1 > h2 && h2 > h3) {
			t.Errorf("rows did not stack with child style %q (y=%.0f,%.0f,%.0f) — percentage height not treated as auto", style, h1, h2, h3)
		}
	}
}
