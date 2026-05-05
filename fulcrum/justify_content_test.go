package fulcrum

import (
	"testing"

	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

// TestJustifyContentShorthand pins the v0.7.1-fulcrum.2 patch:
// `justify-content: end` (the CSS Box Alignment Level 3 shorthand)
// must produce the same result as `flex-end`. align-items and
// align-content already accept the shorthand; before this patch
// justify-content fell through to flex-start, silently mis-aligning
// any page authored with the short form.
//
// We can't easily inspect Flex.justifyContent without reaching into
// the layout package's unexported fields, so we observe the effect at
// the pixel level: an inline-block child in a 100pt-wide flex container
// with `justify-content: end` should be positioned at the right edge,
// not the left. The smoke test is "child X position with `end` matches
// child X position with `flex-end`."
//
// Keeping the assertion at this level (positions match) avoids being
// brittle against folio's internal coordinates while still catching the
// regression: before the patch, `end` lays out at flex-start (left
// edge), so the two PDFs differ by ~80pt.
func TestJustifyContentShorthand(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{name: "flex-end (canonical)", value: "flex-end"},
		{name: "end (shorthand)", value: "end"},
	}

	results := make(map[string][]byte, len(cases))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `<html><head><style>
				body { margin: 0; padding: 0; }
				.row { display: flex; justify-content: ` + tc.value + `; width: 200pt; }
				.box { width: 50pt; height: 20pt; background: #f00; }
			</style></head><body><div class="row"><div class="box"></div></div></body></html>`

			pdf, err := renderHTMLToPDF(src)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			results[tc.value] = pdf
		})
	}

	// flex-end and end must produce byte-identical output. folio is
	// deterministic for the same input, so any difference here means
	// `end` took a different code path than `flex-end`.
	if len(results) == 2 {
		fe := results["flex-end"]
		e := results["end"]
		if len(fe) == 0 || len(e) == 0 {
			t.Fatal("at least one render returned empty bytes")
		}
		if string(fe) != string(e) {
			t.Errorf("`end` produced different PDF than `flex-end` (lengths: end=%d, flex-end=%d) — shorthand is not honored",
				len(e), len(fe))
		}
	}
}

// renderHTMLToPDF is a tiny end-to-end driver for fulcrum tests:
// HTML string → folio document → PDF bytes. Mirrors what
// internal/pdfrender/renderer.go does on the Fulcrum side, but keeps
// the test independent of the layout package's internals.
func renderHTMLToPDF(src string) ([]byte, error) {
	// Use ConvertFull so the @page config (if any) is parsed and
	// applied. For these tiny fixtures, the defaults (US Letter) are
	// fine — we don't care about absolute positions, just byte equality
	// across two inputs that should produce identical layouts.
	res, err := html.ConvertFull(src, &html.Options{PageWidth: 612, PageHeight: 792})
	if err != nil {
		return nil, err
	}
	doc := newDoc()
	for _, e := range res.Elements {
		doc.Add(e)
	}
	// Mirror tmpl.RenderDocument's absolute handling — propagate the
	// BottomAnchored / RightAligned flags so tests of `position:
	// fixed; bottom: 0` and `right: 0` see the production rendering.
	for _, abs := range res.Absolutes {
		doc.AddAbsoluteWithOpts(abs.Element, abs.X, abs.Y, abs.Width, layout.AbsoluteOpts{
			RightAligned:   abs.RightAligned,
			BottomAnchored: abs.BottomAnchored,
			ZIndex:         abs.ZIndex,
			PageIndex:      -1,
		})
	}
	return doc.ToBytes()
}
