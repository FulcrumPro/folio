package fulcrum

import (
	"testing"
)

// TestFontWeight500And600ResolveToBold pins the v0.7.1-fulcrum.9 patch:
// CSS numeric font-weight values 500 and 600 must resolve to "bold"
// when the available font family ships only Regular + Bold variants
// — which is the case for PDF base14 fonts and for the embedded
// @font-face Liberation Sans we ship as the Arial substitute.
//
// CSS Fonts §6.4 says values 100-400 stay normal and 500+ should map
// to bold given the typical "Regular + Bold" pair. folio's
// parseFontWeight previously treated only 700/800/900/bold/bolder as
// bold and silently coerced 500/600 to "normal" — making
// `.data-label-main { font-weight: 600 }` (used by .NET DocGen v3
// for section labels VENDOR / BILLING / PAYMENT TERMS) render
// regular-weight.
//
// We observe the fix at the resolveFontPair / @font-face matching
// layer: a `<span style="font-weight: 600">` should pick up the
// embedded Bold @font-face. We can't easily inspect the embedded
// font choice from outside the html package, so we render the same
// span twice — once with `font-weight: 600` and once with
// `font-weight: bold` — and assert the rendered PDFs are byte-equal.
// Pre-patch they differed because 600 fell through to Regular while
// `bold` mapped to Bold.
func TestFontWeight500And600ResolveToBold(t *testing.T) {
	cases := []struct {
		name     string
		weight   string
		boldName string // value that's known to map to bold (the reference)
	}{
		{name: "weight 600 vs bold", weight: "600", boldName: "bold"},
		{name: "weight 500 vs bold", weight: "500", boldName: "bold"},
		{name: "weight 700 vs bold (regression check)", weight: "700", boldName: "bold"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := `<html><body><p style="font-weight:` + tc.weight + `">test</p></body></html>`
			b := `<html><body><p style="font-weight:` + tc.boldName + `">test</p></body></html>`
			pdfA, err := renderHTMLToPDF(a)
			if err != nil {
				t.Fatalf("render A: %v", err)
			}
			pdfB, err := renderHTMLToPDF(b)
			if err != nil {
				t.Fatalf("render B: %v", err)
			}
			if string(pdfA) != string(pdfB) {
				t.Errorf("font-weight: %s did not produce the same PDF as font-weight: %s. The numeric weight is being mapped to a different rendered weight than the keyword equivalent.",
					tc.weight, tc.boldName)
			}
		})
	}
}
