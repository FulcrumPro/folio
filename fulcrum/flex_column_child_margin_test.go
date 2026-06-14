// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestColumnFlexMinContentIncludesChildMargin pins the v0.9.1-fulcrum.28 patch:
// a column flex's min-content width must include its child's horizontal margins,
// so a grower floored at that min-content (fulcrum.27) keeps the child's margin
// gap to the next column.
//
// The .NET DocGen PurchaseOrderv3 VENDOR box is `width: 300px; margin-right: 8px`
// inside `.v3-info-contain1 { flex: 1; flex-direction: column }`. fulcrum.27
// floored contain1 at the box's 300pt min-content but dropped the 8px margin, so
// the VENDOR and NOTE box borders touched. With the margin included the floor is
// 308pt and the 8px gap is preserved.
func TestColumnFlexMinContentIncludesChildMargin(t *testing.T) {
	src := `<html><head><style>body{margin:0;padding:0;font-size:10pt}
		.wrap{display:flex;width:540pt}
		.c1{flex:1;display:flex;flex-direction:column}
		.c2{flex:2;display:flex;flex-direction:column}
		.boxL{flex-grow:1;width:300px;margin-right:8px;border:1px solid #888;border-radius:2px}
		.boxR{flex-grow:2;display:block;border:1px solid #888;border-radius:2px}
		.inner{padding:3px 8px 8px 8px}
	</style></head><body><div class="wrap">
		<div class="c1"><div>VENDOR</div><div class="boxL"><div class="inner">BILLING</div></div></div>
		<div class="c2"><div>NOTE</div><div class="boxR"><div class="inner">Please confirm</div></div></div>
	</div></body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	noteX := findTextX(pdf, "Please")
	if noteX < 0 {
		t.Fatalf("NOTE text not found")
	}
	t.Logf("NOTE text x=%.1f (300px box + 8px margin + padding/border — must clear ~308)", noteX)
	// Pre-fix (margin dropped) the note started ~303 (box right edge, no gap).
	// With the 8px margin included it starts ~309+. Require it past the box+margin.
	if noteX < 307 {
		t.Errorf("NOTE text x=%.1f — VENDOR box right margin (8px gap) was dropped; boxes touch", noteX)
	}
}
