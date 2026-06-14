// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestStandaloneBrReservesBlankLine pins the v0.9.1-fulcrum.26 patch: a
// standalone <br> between block siblings must reserve one blank line, matching
// the browser (the <br> becomes an anonymous line box one line-height tall).
//
// convertBr built its spacer paragraph from a single space, but splitWords
// runs text through strings.Fields (unicode.IsSpace), which strips a lone
// space (and U+00A0) — leaving a zero-word, zero-height paragraph, so the <br>
// added nothing. The .NET DocGen PurchaseOrder Notes block
// (`<br />{{>dataitem Label='Notes' …}}`) lost the blank line Chrome renders
// above "Notes …".
func TestStandaloneBrReservesBlankLine(t *testing.T) {
	gap := func(src string) float64 {
		full := `<html><head><style>body{margin:0;padding:0;font-size:10pt}</style></head><body>` + src + `</body></html>`
		pdf, err := renderHTMLToPDF(full)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		above := findTextY(pdf, "ABOVE")
		notes := findTextY(pdf, "Notes")
		if above < 0 || notes < 0 {
			t.Fatalf("markers missing: above=%.1f notes=%.1f", above, notes)
		}
		return above - notes // PDF y is top-origin-inverted: bigger gap = more space
	}
	noBr := gap(`<div>ABOVE</div><div class="clear"><div>Notes</div></div>`)
	withBr := gap(`<div>ABOVE</div><div class="clear"><br/><div>Notes</div></div>`)
	added := withBr - noBr
	t.Logf("no-br gap=%.1f  with-br gap=%.1f  <br> added %.1f (expect ≈ one line, ~10-14pt)", noBr, withBr, added)
	if added < 8 {
		t.Errorf("standalone <br> added only %.1fpt — it should reserve ~one blank line", added)
	}
}
