// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestSVGWidthOnlyDerivesHeightFromAspectRatio pins the v0.9.1-fulcrum.24 patch:
// an SVG given only a CSS `width` (no height) must derive its height from the
// viewBox aspect ratio — `width:10px` on a 108×108-viewBox icon is a 10px-tall
// box, not 108px tall.
//
// folio pre-filled the unspecified dimension with the SVG's intrinsic value, so
// the .NET DocGen job-label `.icon` (`.printlabel.small .icon { width:10px }`,
// no height) rendered as a 7.5pt-wide × 108pt-TALL sliver — an invisible thin
// column (logo appeared missing) that also forced its flex `.label-header` row
// to 108pt tall, pushing the centered "Job …" title down with a huge gap.
func TestSVGWidthOnlyDerivesHeightFromAspectRatio(t *testing.T) {
	svgIcon := `<svg class="icon" viewBox="0 0 108 108" style="enable-background:new 0 0 108 108;"><style type="text/css">.st4{fill:#135EAB}</style><g><path class="st4" d="M104.1,4.2H87c-1.5,0-3.2,0.8-4.2,2.6z"/></g></svg>`
	css := `.printlabel.small .label-header{display:flex;align-items:center}` +
		`.printlabel.small .icon{width:10px;min-width:10px;margin-right:2px}`
	src := `<html><head><style>body{margin:0;padding:0}` + css + `</style></head><body>` +
		`<div class="printlabel small"><div class="label-header">` + svgIcon +
		`<div class="title">Job JOB-2042</div></div>` +
		`<div>BELOW</div></div></body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// The icon is ~10px ≈ 7.5pt. The flex row must be ~icon-height tall, so the
	// "BELOW" marker should sit close to the title, not ~100pt down. Measure the
	// vertical gap from the title to BELOW.
	title := findTextY(pdf, "Job")
	below := findTextY(pdf, "BELOW")
	if title < 0 || below < 0 {
		t.Fatalf("markers missing: title=%.1f below=%.1f", title, below)
	}
	gap := title - below
	t.Logf("title y=%.1f  below y=%.1f  gap=%.1f (was ~108 when SVG height stayed intrinsic)", title, below, gap)
	if gap > 40 {
		t.Errorf("label-header row gap=%.1f — SVG height did not collapse to the CSS-width aspect ratio (row inflated to viewBox height)", gap)
	}
}
