// Package fulcrum holds regression tests for Fulcrum-specific patches
// against folio. Tests live here rather than in upstream's html/*_test.go
// files so the patch diff against carlos7ags/main stays small and obvious.
//
// Each patch ships with at least one focused fixture demonstrating the
// behavior the patch unlocks. The fixtures are intentionally minimal —
// the goal is "this exact CSS spec point now works", not "PO PDFs render
// pixel-perfect" (that's the Fulcrum repo's golden tests' job).
package fulcrum

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

// TestInlineWhitespaceBetweenInlineSiblings pins the v0.7.1-fulcrum.1
// patch: whitespace-only text nodes between adjacent inline siblings
// (e.g. `<span>A</span>\n  <span>B</span>`) must keep the spans grouped
// in a single inline paragraph instead of splitting them into separate
// anonymous block boxes — CSS Text Module Level 3 §4.1.1.
//
// Before the patch, walkChildren flushed the inline buffer on the
// whitespace text node, so a single source line of two adjacent spans
// rendered as two paragraphs stacked vertically. Browsers (and
// jsreport's Chromium) collapse the inter-element whitespace to one
// inter-word space and keep the spans on one line.
//
// We observe the fix at the layout level: the inline siblings should
// produce exactly one *layout.Paragraph rather than one paragraph per
// span. Folio unwraps a bare <div> with text content into a Paragraph
// at the top level, so the body's top-level layout list is a direct
// view of the patch's effect.
func TestInlineWhitespaceBetweenInlineSiblings(t *testing.T) {
	cases := []struct {
		name    string
		htmlSrc string
	}{
		{
			name:    "two spans separated by newline+spaces",
			htmlSrc: "<html><body><div><span>Hello</span>\n  <span>World</span></div></body></html>",
		},
		{
			name:    "three spans with whitespace between each",
			htmlSrc: "<html><body><div><span>A</span>  <span>B</span>\n<span>C</span></div></body></html>",
		},
		{
			name:    "strong then space then em (mixed inline tags)",
			htmlSrc: "<html><body><div><strong>Bold</strong> <em>italic</em></div></body></html>",
		},
		{
			name:    "data-label / data-value pattern (Fulcrum dataitem partial)",
			htmlSrc: `<html><body><div class="data-item"><span class="data-label">Phone</span> <span class="data-value">555-1234</span></div></body></html>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			elems, err := html.Convert(tc.htmlSrc, &html.Options{PageWidth: 612, PageHeight: 792})
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			got := countParagraphs(elems)
			if got != 1 {
				t.Errorf("paragraphs: got %d, want 1 (top level: %s)",
					got, elementSummary(elems))
			}
		})
	}
}

// TestInlineWhitespaceBetweenBlockSiblings is a regression net for the
// other side of the patch: whitespace text between two block siblings
// must NOT be promoted to inline flow. Otherwise we'd insert a stray
// spacer paragraph between every pair of <div>s.
func TestInlineWhitespaceBetweenBlockSiblings(t *testing.T) {
	src := "<html><body>\n  <div>A</div>\n  <div>B</div>\n</body></html>"
	elems, err := html.Convert(src, &html.Options{PageWidth: 612, PageHeight: 792})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	// Each <div>A</div> unwraps to one Paragraph; we want exactly two,
	// with no spacer paragraph for the whitespace between them or at
	// the document boundaries.
	got := countParagraphs(elems)
	if got != 2 {
		t.Errorf("paragraphs: got %d, want 2 (top level: %s)",
			got, elementSummary(elems))
	}
}

// TestInlineWhitespaceBoundaryDropping confirms whitespace at the start
// or end of a parent (no inline sibling on one side) is dropped, not
// promoted to a leading or trailing space paragraph.
func TestInlineWhitespaceBoundaryDropping(t *testing.T) {
	src := "<html><body><div>\n  <span>only-child</span>\n  </div></body></html>"
	elems, err := html.Convert(src, &html.Options{PageWidth: 612, PageHeight: 792})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := countParagraphs(elems)
	if got != 1 {
		t.Errorf("paragraphs: got %d, want 1 (top level: %s)",
			got, elementSummary(elems))
	}
}

func countParagraphs(elems []layout.Element) int {
	n := 0
	for _, e := range elems {
		if _, ok := e.(*layout.Paragraph); ok {
			n++
		}
	}
	return n
}

// elementSummary names the type of every element in a slice — used for
// failure messages so a count mismatch is debuggable without rerunning
// under a debugger.
func elementSummary(elems []layout.Element) string {
	parts := make([]string, len(elems))
	for i, e := range elems {
		switch e.(type) {
		case *layout.Paragraph:
			parts[i] = "Paragraph"
		case *layout.Div:
			parts[i] = "Div"
		case *layout.Float:
			parts[i] = "Float"
		default:
			parts[i] = "?"
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
