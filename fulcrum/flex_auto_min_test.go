// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestFlexGrowerFlooredAtMinContent pins the v0.9.1-fulcrum.27 patch: a growing
// flex item (flex-grow > 0, min-width auto) must not be allocated less than its
// min-content size (CSS Flexbox §4.5 automatic minimum size). Otherwise a
// definite-width child overflows the item and intrudes on the next column.
//
// The .NET DocGen PurchaseOrderv3 header is a flex row: `.v3-info-contain1
// { flex: 1 }` (VENDOR) holding `.v3-pdf-details-left-po { width: 300px }`, and
// `.v3-info-contain2 { flex: 2 }` (NOTE). folio gave contain1 only its 1/3 grow
// share (~180pt), so the 300px VENDOR box overflowed right and its border cut
// through the NOTE text. With the floor, contain1 is held at >= 300pt and the
// NOTE column begins after it.
func TestFlexGrowerFlooredAtMinContent(t *testing.T) {
	src := `<html><head><style>body{margin:0;padding:0;font-size:10pt}
		.wrap{display:flex;width:540pt}
		.c1{flex:1;display:flex;flex-direction:column}
		.c2{flex:2;display:flex;flex-direction:column}
		.boxL{flex-grow:1;width:300px;margin-right:8px;border:1px solid #888;border-radius:2px}
		.boxR{flex-grow:2;display:block;border:1px solid #888;border-radius:2px}
		.inner{padding:3px 8px 8px 8px}
	</style></head><body><div class="wrap">
		<div class="c1"><div>VENDOR</div><div class="boxL"><div class="inner">BILLING</div></div></div>
		<div class="c2"><div>NOTE</div><div class="boxR"><div class="inner">Please confirm receipt</div></div></div>
	</div></body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	noteX := findTextX(pdf, "Please")
	if noteX < 0 {
		t.Fatalf("NOTE text not found")
	}
	t.Logf("NOTE text x=%.1f (VENDOR box width 300px — note must start at/after it, not overlap)", noteX)
	// The 300px VENDOR box (+ its right margin) ends near x≈300-308. Pre-fix the
	// note started ~258 (overlapping the box). Require it to clear the box.
	if noteX < 295 {
		t.Errorf("NOTE text x=%.1f overlaps the 300px VENDOR box — grower not floored at min-content", noteX)
	}
}
