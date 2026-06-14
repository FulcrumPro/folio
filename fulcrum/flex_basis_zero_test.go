// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestFlexBasisZeroGrows pins the v0.9.1-fulcrum.15 patch: an explicit
// `flex-basis: 0` must be treated as a real base size of zero (the item's size
// comes purely from flex-grow), distinct from `flex-basis: auto` (which falls
// back to the content's max-content size).
//
// folio's row basis resolution gated on `effectiveBasis() > 0`, so an explicit
// `flex-basis: 0` resolved to 0, failed the `> 0` test, and fell through to the
// max-content path — i.e. it was treated as `auto`. For a `flex-basis: 0;
// flex-grow: N` item whose content is long, that bogus max-content basis made
// the row overflow, so the items *shrank* by content weight instead of
// *growing* in their N:M proportions.
//
// The .NET DocGen CAPA/NCR quality reports lay out a two-column data block as
//
//	.data-container          { display: flex }
//	.data-container .left    { flex-grow: 1; flex-basis: 0 }
//	.data-container .right   { flex-grow: 2; flex-basis: 0 }
//
// expecting a 1:2 (1/3 : 2/3) split. The right column holds long paragraphs
// (Problem Statement, Root Cause Analysis). Pre-patch the left column collapsed
// to ~1/8 of the row and its text wrapped word-by-word; with the patch the
// columns land at the 1/3 : 2/3 boundary Chrome produces.
func TestFlexBasisZeroGrows(t *testing.T) {
	const W = 564 // container width in pt
	src := `<html><head><style>
		body{margin:0;padding:0;font-size:9pt}
		.row{display:flex;width:` + itoa(W) + `pt}
		.row .left{flex-grow:1;flex-basis:0}
		.row .right{flex-grow:2;flex-basis:0}
	</style></head><body>
	<div class="row">
		<div class="left"><span>LEFTMARK</span></div>
		<div class="right"><span>RIGHTMARK</span> a long paragraph of text that would otherwise produce a huge max-content basis and make folio shrink the columns by content weight instead of growing them in the authored one to two proportion across the row.</div>
	</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	xl := findTextX(pdf, "LEFTMARK")
	xr := findTextX(pdf, "RIGHTMARK")
	leftWidth := xr - xl
	// flex-grow 1:2 over a 564pt row → left column ≈ 1/3 ≈ 188pt. Allow slack
	// for the default page margin baseline and rounding.
	if leftWidth < 150 || leftWidth > 220 {
		t.Errorf("left column width=%.1f (right starts at x=%.1f) — expected ~188 (1/3 of %dpt via flex-grow 1:2); flex-basis:0 was treated as auto",
			leftWidth, xr, W)
	}
}

// itoa avoids pulling strconv into the test for one int.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
