// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestWidthFitContentShrinks pins the v0.9.1-fulcrum.16 patch: `width:
// fit-content` (and min-content / max-content) must shrink the box to its
// content instead of filling the containing block.
//
// folio's `width` parser only understood lengths / percentages / auto;
// `fit-content` parsed to nil (== auto), so a block with a background and
// `width: fit-content` stretched to the full container width.
//
// The .NET DocGen v3 header uses `.title-background { width: fit-content }` so
// the colored title plate hugs "Purchase Order" / "Invoice" / etc. Pre-patch
// the plate stretched across the whole header column.
func TestWidthFitContentShrinks(t *testing.T) {
	// A fit-content plate sits to the LEFT of the page; a right-floated sibling
	// marker is placed after it in normal flow. With fit-content the plate
	// hugs its text, so a following inline sibling on the same line would sit
	// near the text end — but the simplest robust signal is that fit-content
	// output differs from auto (full-width) output. If fit-content were still
	// treated as auto the two renders would be byte-identical.
	mk := func(w string) string {
		return `<html><head><style>
			body{margin:0;padding:0}
			.plate{background:#273e6c;width:` + w + `;padding:3pt 12pt;color:#fff}
		</style></head><body>
			<div class="plate">TITLE</div>
		</body></html>`
	}
	fit, err := renderHTMLToPDF(mk("fit-content"))
	if err != nil {
		t.Fatalf("render fit: %v", err)
	}
	auto, err := renderHTMLToPDF(mk("auto"))
	if err != nil {
		t.Fatalf("render auto: %v", err)
	}
	if string(fit) == string(auto) {
		t.Error("width:fit-content produced the same output as width:auto — fit-content was treated as auto (box stretched full width)")
	}
	if findTextY(fit, "TITLE") < 0 {
		t.Error("TITLE missing from fit-content render")
	}
}

// TestWidthFitContentText keeps the fit-content box from stretching: a marker
// placed in normal flow immediately after the plate's text (via inline-block)
// lands right after the short title, not pushed to the next line by a
// full-width plate.
func TestWidthFitContentText(t *testing.T) {
	// Two inline-block boxes side by side: a fit-content plate and a marker.
	// If the plate hugs its content the marker sits just past it (small x);
	// if the plate stretched full width the marker would wrap to a new line.
	src := `<html><head><style>
		body{margin:0;padding:0}
		.plate{display:inline-block;background:#273e6c;width:fit-content;padding:2pt 8pt;color:#fff}
		.mark{display:inline-block}
	</style></head><body><span class="plate">HI</span><span class="mark">MARK</span></body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	yh := findTextY(pdf, "HI")
	ym := findTextY(pdf, "MARK")
	if yh < 0 || ym < 0 {
		t.Fatalf("missing text: HI=%.1f MARK=%.1f", yh, ym)
	}
	// Same baseline => same line => plate did not stretch full width.
	if yh-ym > 3 || ym-yh > 3 {
		t.Errorf("MARK (y=%.1f) not on the same line as HI (y=%.1f) — fit-content plate stretched and pushed the marker down", ym, yh)
	}
}
