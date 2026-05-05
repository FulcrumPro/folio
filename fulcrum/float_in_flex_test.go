package fulcrum

import (
	"bytes"
	"testing"
)

// TestFloatIgnoredInsideFlex pins the v0.7.1-fulcrum.5 patch:
// per CSS Flexbox §3, the `float` property has no effect on direct
// flex children — a flex item with `float: left` should lay out as
// if no float were declared. folio v0.7.1's convertElement
// unconditionally wraps floated elements in a layout.Float box, and
// the flex layout then mis-computed item widths because a Float
// wrapper isn't a sensible flex item.
//
// Symptom: .NET DocGen's `.data-container { display: flex }` with
// `.three-columns { float: left }` children mis-shrunk the columns
// far below their 33% width, forcing aggressive text wrap inside
// each column. Browsers (and jsreport's Chromium) ignore the float
// inside flex per spec.
//
// We observe the fix by rendering identical HTML twice — once with
// `float: left` declared on the flex children, once with the float
// removed — and asserting byte-equal PDFs. With the patch, the
// presence of `float: left` on a flex item must be a no-op.
func TestFloatIgnoredInsideFlex(t *testing.T) {
	// Use long text that would wrap differently in a mis-shrunk
	// column. The .three-columns case in .NET DocGen is the canonical
	// mis-rendering: float:left on a 33% flex child causes folio to
	// allocate a tiny column, forcing many extra line wraps.
	body := `<div class="row">
			<div class="col">The quick brown fox jumps over the lazy dog repeatedly across the meadow.</div>
			<div class="col">The quick brown fox jumps over the lazy dog repeatedly across the meadow.</div>
			<div class="col">The quick brown fox jumps over the lazy dog repeatedly across the meadow.</div>
		</div>`
	withFloat := `<html><head><style>
		.row { display: flex; width: 600pt; }
		.col { width: 33%; float: left; padding-right: 10px; }
	</style></head><body>` + body + `</body></html>`

	withoutFloat := `<html><head><style>
		.row { display: flex; width: 600pt; }
		.col { width: 33%; padding-right: 10px; }
	</style></head><body>` + body + `</body></html>`

	withPDF, err := renderHTMLToPDF(withFloat)
	if err != nil {
		t.Fatalf("render with-float: %v", err)
	}
	withoutPDF, err := renderHTMLToPDF(withoutFloat)
	if err != nil {
		t.Fatalf("render without-float: %v", err)
	}
	if !bytes.Equal(withPDF, withoutPDF) {
		t.Errorf("`float: left` on a flex child changed the output (with-float=%d bytes, without-float=%d bytes) — float should be a no-op inside flex containers per CSS spec",
			len(withPDF), len(withoutPDF))
	}
}
