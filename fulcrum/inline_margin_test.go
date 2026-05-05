package fulcrum

import (
	"bytes"
	"testing"
)

// TestInlineMarginProducesGap pins the v0.7.1-fulcrum.4 patch:
// horizontal margin on inline elements (`<span style="margin-right:
// 5px">`) must produce a visible gap between the element and the
// surrounding text. CSS spec: margin applies to inline boxes
// horizontally (top/bottom are ignored on inline). folio v0.7.1's
// inline path (collectRunsFromNode) skipped margin entirely, so a
// `<span class="data-label">Phone</span><span>555-1234</span>` shaped
// after the .NET DocGen `dataitem.hbs` rendered as
// "Phone555-1234" with no gap.
//
// The patch emits a single whitespace TextRun before/after each inline
// element with margin-left / margin-right > 0, with LetterSpacing set
// to the requested margin so paragraph layout reserves visible
// separation. Exact font-metric calibration is impractical without the
// run measurer (only available at layout time), so the rendered gap
// is approximate — but >= 0, which is the regression we care about.
//
// We observe the fix by rendering identical content twice — once with
// margin-right declared, once with explicit whitespace inserted in
// the HTML — and asserting the patched margin path produces a wider
// PDF than the un-margined baseline. We don't assert byte-equality
// against the explicit-whitespace baseline because the LetterSpacing
// approximation differs from a literal space's natural advance.
func TestInlineMarginProducesGap(t *testing.T) {
	// No margin: spans render butted together, "AB" with no gap.
	noGap := `<html><body><p><span>A</span><span>B</span></p></body></html>`

	// margin-right on the first span. Patch should reserve gap.
	withMargin := `<html><body><p><span style="margin-right: 5px">A</span><span>B</span></p></body></html>`

	noGapPDF, err := renderHTMLToPDF(noGap)
	if err != nil {
		t.Fatalf("render no-gap: %v", err)
	}
	withMarginPDF, err := renderHTMLToPDF(withMargin)
	if err != nil {
		t.Fatalf("render with-margin: %v", err)
	}

	if bytes.Equal(noGapPDF, withMarginPDF) {
		t.Error("`margin-right: 5px` produced byte-identical PDF to no-margin — folio is still ignoring inline margin")
	}
}

// TestInlineMarginLeftAlsoProducesGap covers the symmetric
// `margin-left` case. The patch handles both directions, but margins
// usually appear on the leading element via margin-right; verify
// margin-left also works for templates that style the trailing element.
func TestInlineMarginLeftAlsoProducesGap(t *testing.T) {
	noGap := `<html><body><p><span>A</span><span>B</span></p></body></html>`
	withMargin := `<html><body><p><span>A</span><span style="margin-left: 5px">B</span></p></body></html>`

	noGapPDF, err := renderHTMLToPDF(noGap)
	if err != nil {
		t.Fatalf("render no-gap: %v", err)
	}
	withMarginPDF, err := renderHTMLToPDF(withMargin)
	if err != nil {
		t.Fatalf("render with-margin: %v", err)
	}
	if bytes.Equal(noGapPDF, withMarginPDF) {
		t.Error("`margin-left: 5px` produced byte-identical PDF to no-margin — folio is still ignoring inline margin")
	}
}
