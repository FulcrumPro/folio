// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// Render-level regression tests for issue #329 / #340. The structural tests in
// the html package assert the converted element tree is correct (e.g. the inner
// paragraph background is nil). They are necessary but not sufficient: the box
// background can still leak into the rendered content stream as a square fill
// via a different field (a per-run TextRun.BackgroundColor highlight, painted by
// the renderer as a plain ` re` rectangle behind the text). These tests render
// HTML to an actual PDF, decompress the page content stream, and assert that a
// box with a border-radius and background color C draws the rounded path (Bezier
// curve operators ` c`) for C and does NOT draw a plain rectangle (` re` ... ` f`)
// fill in color C — i.e. there is exactly one rounded fill and no square
// overpaint.

// squareFillRe matches a PDF rectangle path: "x y w h re".
var squareFillRe = regexp.MustCompile(`(?m)^\s*-?\d[\d.]*\s+-?\d[\d.]*\s+-?\d[\d.]*\s+-?\d[\d.]*\s+re\s*$`)

// squareFillsInColor scans a decompressed content stream for ` re` rectangle
// paths emitted while the current non-stroking color equals colorPrefix (the
// "<r> <g> <b> rg" line). It returns each offending rectangle line so failures
// are legible. Stroking-color changes ("RG") do not affect the fill color, so
// only "rg" lines update the tracked color.
func squareFillsInColor(cs, colorPrefix string) []string {
	var out []string
	curColor := ""
	for _, l := range strings.Split(cs, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasSuffix(t, " rg") {
			curColor = t
		}
		if strings.HasPrefix(curColor, colorPrefix) && squareFillRe.MatchString(l) {
			out = append(out, t)
		}
	}
	return out
}

// hasRoundedFillInColor reports whether the stream contains a Bezier curve
// operator (` c`) while the current fill color equals colorPrefix — the
// signature of a rounded-rectangle fill in that color.
func hasRoundedFillInColor(cs, colorPrefix string) bool {
	curColor := ""
	for _, l := range strings.Split(cs, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasSuffix(t, " rg") {
			curColor = t
		}
		if strings.HasPrefix(curColor, colorPrefix) && strings.HasSuffix(t, " c") {
			return true
		}
	}
	return false
}

func renderHTMLStream(t *testing.T, html string) string {
	t.Helper()
	doc := NewDocument(PageSizeLetter)
	if err := doc.AddHTML(html, nil); err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return decompressedContentStreams(t, buf.Bytes())
}

func TestBorderRadiusNoSquareOverpaint(t *testing.T) {
	// #4F46E5 -> 0.309804 0.27451 0.898039 ; #FEF3C7 -> 0.996078 0.952941 0.780392
	const indigo = "0.309804 0.27451 0.898039"
	const amber = "0.996078 0.952941 0.780392"

	cases := []struct {
		name  string
		html  string
		color string
	}{
		{
			name:  "FlexSpanChip",
			html:  `<div style="display:flex"><span style="flex:0 0 auto;background:#4F46E5;color:#fff;border-radius:8pt;padding:5pt 12pt">CHIP</span></div>`,
			color: indigo,
		},
		{
			name:  "GridSpanChip",
			html:  `<div style="display:grid;grid-template-columns:1fr"><span style="background:#4F46E5;color:#fff;border-radius:8pt;padding:5pt 12pt">CHIP</span></div>`,
			color: indigo,
		},
		{
			name:  "DisplayBlockSpan",
			html:  `<span style="display:block;background:#4F46E5;color:#fff;border-radius:8pt;padding:5pt 12pt">CHIP</span>`,
			color: indigo,
		},
		{
			name:  "PercentRadiusFlexSpan",
			html:  `<div style="display:flex"><span style="flex:0 0 auto;background:#4F46E5;color:#fff;border-radius:50%;padding:5pt 12pt">CHIP</span></div>`,
			color: indigo,
		},
		{
			name:  "Blockquote",
			html:  `<blockquote style="background:#FEF3C7;border-radius:10pt;padding:8pt 12pt;margin:0">quoted</blockquote>`,
			color: amber,
		},
		{
			name:  "Figure",
			html:  `<figure style="background:#FEF3C7;border-radius:10pt;padding:8pt 12pt;margin:0">cap</figure>`,
			color: amber,
		},
		{
			name:  "TableWrapper",
			html:  `<table style="background:#FEF3C7;border-radius:10pt"><tr><td>cell</td></tr></table>`,
			color: amber,
		},
		{
			// #333's plain block Div case carried the same latent run-highlight
			// leak (text inside the div inherits the div background onto its run).
			name:  "PlainDivWithText",
			html:  `<div style="background:#4F46E5;color:#fff;border-radius:8pt;padding:5pt 12pt">BOX</div>`,
			color: indigo,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := renderHTMLStream(t, tc.html)
			if !hasRoundedFillInColor(cs, tc.color) {
				t.Errorf("expected a rounded fill (curve ops) in color %s; stream:\n%s", tc.color, cs)
			}
			if sq := squareFillsInColor(cs, tc.color); len(sq) > 0 {
				t.Errorf("found square `re` fill(s) in box color %s (square overpaint over rounded fill): %v\nstream:\n%s", tc.color, sq, cs)
			}
		})
	}
}

// TestLiBorderRadiusNoSquareOverpaint guards the #343 fix: a styled <li> routed
// through the element path builds a Div that paints a rounded (or circular) fill,
// but the <li>'s background also propagated onto the inner content paragraph and
// its text runs. Without clearing those, they re-draw a square `re` fill over the
// rounded fill, squaring off the corners. This mirrors TestBorderRadiusNoSquareOverpaint
// for the list-item path and FAILS before the fix (the badge emitted a full-box
// and a run-highlight indigo square).
func TestLiBorderRadiusNoSquareOverpaint(t *testing.T) {
	// #4F46E5 -> 0.309804 0.27451 0.898039
	const indigo = "0.309804 0.27451 0.898039"

	cases := []struct {
		name  string
		html  string
		color string
	}{
		{
			name:  "InlineBlockCircleBadge",
			html:  `<ul><li style="display:inline-block;background:#4F46E5;color:#fff;border-radius:50%;width:28px;height:28px;text-align:center">4</li></ul>`,
			color: indigo,
		},
		{
			name:  "RoundedSingleLine",
			html:  `<ul><li style="border-radius:12px;background:#4F46E5;color:#fff;padding:4px 10px">Rounded item</li></ul>`,
			color: indigo,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := renderHTMLStream(t, tc.html)
			if !hasRoundedFillInColor(cs, tc.color) {
				t.Errorf("expected a rounded fill (curve ops) in color %s; stream:\n%s", tc.color, cs)
			}
			if sq := squareFillsInColor(cs, tc.color); len(sq) > 0 {
				t.Errorf("found square `re` fill(s) in box color %s (square overpaint over rounded <li> fill): %v\nstream:\n%s", tc.color, sq, cs)
			}
		})
	}
}

// grayFillStripeInColor reports whether the stream fills a ` re` rectangle
// stripe while the current non-stroking color is gray (0.6 0.6 0.6) — the
// signature of the inner accent fill. It also returns the matching ` re` line.
func grayFillStripeInColor(cs string) (bool, string) {
	curColor := ""
	for _, l := range strings.Split(cs, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasSuffix(t, " rg") {
			curColor = t
		}
		if strings.HasPrefix(curColor, "0.6 0.6 0.6") && squareFillRe.MatchString(l) {
			return true, t
		}
	}
	return false, ""
}

// TestBlockquoteRoundedAccentInnerFill verifies the issue #329/#340 fix:
// a blockquote with an explicit border-radius KEEPS its default gray left accent,
// but the accent now renders as an INNER FILLED stripe (a ` re` rectangle filled
// in gray) clipped to the rounded box outline — flush with the left edge and
// rounded at the top-left/bottom-left corners — instead of a centered stroke
// along the outline that read as a detached outer bracket. There must be NO
// stroked gray accent (0.6 0.6 0.6 RG / S) for the rounded case. A normal
// blockquote (no radius) keeps its straight STROKED accent unchanged.
func TestBlockquoteRoundedAccentInnerFill(t *testing.T) {
	rounded := renderHTMLStream(t,
		`<blockquote style="background:#FEF3C7;border-radius:10pt;padding:8pt 12pt;margin:0">quoted</blockquote>`)

	// The rounded accent is an inner filled stripe: a gray ` re` rectangle fill.
	ok, stripe := grayFillStripeInColor(rounded)
	if !ok {
		t.Fatalf("rounded blockquote lost its gray inner accent fill (no gray `re` stripe); stream:\n%s", rounded)
	}
	// The stripe must be a thin left bar at the box's left edge (x=72, w=3),
	// not a full-width fill.
	if !strings.HasPrefix(stripe, "72 ") || !strings.Contains(stripe, " 3 ") {
		t.Errorf("rounded accent stripe is not a 3pt-wide left bar at x=72: %q\nstream:\n%s", stripe, rounded)
	}
	// It must NOT stroke the accent along the outline for the rounded case.
	if strings.Contains(rounded, "0.6 0.6 0.6 RG") {
		t.Errorf("rounded blockquote must NOT stroke the gray accent (0.6 0.6 0.6 RG); stream:\n%s", rounded)
	}
	// The accent must be clipped to the rounded shape: a clip op (W n) precedes
	// the gray fill, so the corners round.
	if !strings.Contains(rounded, "W\nn") && !strings.Contains(rounded, "W n") {
		t.Errorf("rounded accent fill is not clipped to the rounded shape (no `W n`); stream:\n%s", rounded)
	}

	plain := renderHTMLStream(t, `<blockquote>quoted</blockquote>`)
	// The plain accent (no radius) is a straight STROKED bar.
	if !strings.Contains(plain, "0.6 0.6 0.6 RG") {
		t.Fatalf("plain blockquote lost its straight stroked gray accent (0.6 0.6 0.6 RG); stream:\n%s", plain)
	}
	// And the plain case must NOT use a gray fill stripe.
	if ok, _ := grayFillStripeInColor(plain); ok {
		t.Errorf("plain blockquote unexpectedly drew a gray FILL stripe instead of a stroke; stream:\n%s", plain)
	}
}

// TestBorderRadiusPreservesUnrelatedInlineHighlight ensures the run-background
// clearing only removes the box-colored highlight, not an inline <span>
// highlight of a different color inside the same rounded container.
func TestBorderRadiusPreservesUnrelatedInlineHighlight(t *testing.T) {
	cs := renderHTMLStream(t,
		`<div style="background:#4F46E5;border-radius:8pt;padding:10pt">hi <span style="background:#FFFF00">YEL</span> bye</div>`)
	if !strings.Contains(cs, "1 1 0 rg") {
		t.Errorf("yellow inline highlight (1 1 0 rg) was incorrectly cleared; stream:\n%s", cs)
	}
}
