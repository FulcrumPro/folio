package fulcrum

import (
	"bytes"
	"testing"
)

// TestInlineFloatPromotesToBlock pins the v0.7.1-fulcrum.6 patch:
// per CSS 2.1 §9.7, setting `float` to a non-`none` value on an
// element forces its computed display to `block`. folio v0.7.1's
// isInlineFlowElement returned true for `<span>` regardless of float,
// so a floated span stayed in the parent's inline buffer, was rendered
// inline as plain text, and the float declaration silently dropped.
//
// Symptom in production: .NET DocGen's
// `<span style="float:right;">Created By …</span>` rendered at the
// left margin instead of floating to the right edge.
//
// We observe the fix by rendering identical text twice — once with
// `float: right` declared on the span, once without — and asserting
// the PDFs differ. With the patch, the floated span lays out at a
// different X position than the inline span, producing different
// content streams. (Without the patch, both render identically as
// plain inline text.)
func TestInlineFloatPromotesToBlock(t *testing.T) {
	withFloat := `<html><body>
		<div>Heading <span style="float:right">Right Side</span></div>
	</body></html>`
	withoutFloat := `<html><body>
		<div>Heading <span>Right Side</span></div>
	</body></html>`

	withPDF, err := renderHTMLToPDF(withFloat)
	if err != nil {
		t.Fatalf("render with-float: %v", err)
	}
	withoutPDF, err := renderHTMLToPDF(withoutFloat)
	if err != nil {
		t.Fatalf("render without-float: %v", err)
	}
	if bytes.Equal(withPDF, withoutPDF) {
		t.Errorf("`float: right` on a span produced byte-identical PDF to no-float — folio still renders the floated span inline at the left margin")
	}
}
