// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestBlockNegativeMarginLeftShifts pins the v0.9.1-fulcrum.30 patch: a non-auto
// CSS `margin-left` on a block (including a negative value) must shift the box.
//
// convertBlock applied margin-top/bottom and auto (centering) margins but
// dropped a literal margin-left. The .NET DocGen v3 `.title-background` plate
// uses `margin-left: -34px` to bleed off the left edge, with a compensating
// `padding-left: 30px`; folio dropped the bleed, so the title text ("Quote" /
// "Sales Order") sat ~34px too far right with excess left padding vs Chrome.
func TestBlockNegativeMarginLeftShifts(t *testing.T) {
	mk := func(ml string) string {
		return `<html><head><style>body{margin:0;padding:0}
			.hl{display:block;padding-left:9px}
			.tb{background:#ccc;width:fit-content;padding:3px 12px 3px 30px;margin-left:` + ml + `}
			.t{font-size:24pt}
		</style></head><body><div class="hl"><div class="tb"><div class="t">Quote</div></div></div></body></html>`
	}
	x := func(ml string) float64 {
		pdf, err := renderHTMLToPDF(mk(ml))
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return findTextX(pdf, "Quote")
	}
	x0 := x("0px")
	xNeg := x("-34px")
	shift := xNeg - x0
	t.Logf("Quote x: margin-left:0=%.1f  -34px=%.1f  shift=%.1f (expect ~ -25.5pt = -34px)", x0, xNeg, shift)
	// -34px ≈ -25.5pt. Require the title shifted left by roughly that.
	if shift > -20 {
		t.Errorf("negative margin-left shifted the title by only %.1fpt — block margin-left not applied (title plate bleed lost)", shift)
	}
}
