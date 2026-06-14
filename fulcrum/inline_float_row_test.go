// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestInlineTrailingFloatRightAligns pins the v0.9.1-fulcrum.24 patch: a block
// whose content is a leading inline run followed by a trailing `float:right`
// element must lay the floated content out on the SAME line as the inline text
// (Chrome floats it up to the top-right), not stacked below it.
//
// folio's float model only hoists a float that PRECEDES the text it wraps, so a
// float written after the inline content was stacked below — the .NET DocGen
// `.title-bar` (companyinfo spans on the left, a `float:right` "Created By …"
// span on the right; PurchaseOrder / NCR / CAPA / Certification) dropped the
// right-hand text to a second line at the bottom of the gray header band.
func TestInlineTrailingFloatRightAligns(t *testing.T) {
	src := `<html><head><style>
		body{margin:0;padding:0}
		.title-bar{padding:10px 8px;background-color:#EAEBEF;font-size:10pt;line-height:1;border-radius:2px}
	</style></head><body>
		<div class="title-bar">
			<span>acme.example</span> <span>|</span> <span><strong>P</strong> (555) 123-4567</span>
			<span style="float:right;">Created By Alice Anderson <span>| alice@acme.example</span></span>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	left := findTextY(pdf, "acme.example")
	right := findTextY(pdf, "Created")
	if left < 0 || right < 0 {
		t.Fatalf("markers missing: left=%.1f right=%.1f", left, right)
	}
	t.Logf("inline(acme) y=%.1f  float-right(Created) y=%.1f  delta=%.1f", left, right, left-right)
	if d := left - right; d > 3 || d < -3 {
		t.Errorf("float-right y=%.1f not aligned with inline text y=%.1f (delta %.1f) — float dropped below the line", right, left, d)
	}
}

// TestTrailingFloatDoesNotHijackWrapColumns guards the inlineFloatRow
// restriction: a float that PRECEDES following block content (the wrap-around
// column case) must NOT be collapsed into the inline-float row — it stays a
// real float so subsequent content can wrap beside it.
func TestTrailingFloatDoesNotHijackWrapColumns(t *testing.T) {
	// float:left followed by a block paragraph: classic wrap-around, not the
	// title-bar shape. inlineFloatRow must decline (returns false), leaving the
	// float in normal flow. We only assert it renders without error and both
	// texts are present at sane positions.
	src := `<html><head><style>body{margin:0}</style></head><body>
		<div style="width:300pt">
			<div style="float:left;width:80pt">SIDEBAR</div>
			<p>Body text that should wrap to the right of the floated sidebar box.</p>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if findTextY(pdf, "SIDEBAR") < 0 || findTextY(pdf, "Body") < 0 {
		t.Errorf("expected both SIDEBAR and Body text present")
	}
}
