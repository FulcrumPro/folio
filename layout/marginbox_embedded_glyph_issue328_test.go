// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// The only embeddable font fixture checked into the repo is
// font/testdata/synthetic_cjk.ttf, which covers a handful of CJK
// ideographs and nothing else. Its cmap maps every ASCII codepoint to
// GID 0 (.notdef), so a margin box containing ASCII page numbers
// ("Page 1") would encode to an all-zero GID stream and render INVISIBLE
// even though the font is embedded and PDF/A-valid. These tests
// therefore drive the margin-box draw path with content the fixture
// actually covers (CJK runes → non-zero GIDs) so the GID stream is
// provably non-.notdef. The matching ASCII-coverage guard lives in the
// html package, where the production font-resolution path is exercised
// end to end with whatever glyphs the body font supplies.

// loadSyntheticCJKEmbedded loads the synthetic CJK fixture as an
// EmbeddedFont for use as a margin-box body font.
func loadSyntheticCJKEmbedded(t *testing.T) *font.EmbeddedFont {
	t.Helper()
	path, err := filepath.Abs("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	face, err := font.LoadFont(path)
	if err != nil {
		t.Fatalf("load fixture face: %v", err)
	}
	return font.NewEmbeddedFont(face)
}

// hexTjRe captures the GID bytes of an "<HEX> Tj" operator emitted by
// the embedded (Identity-H) text path.
var hexTjRe = regexp.MustCompile(`<([0-9A-Fa-f]+)> Tj`)

// renderEmbeddedMarginBox renders a single page with one bottom-center
// margin box drawn with the supplied embedded font and returns the
// page content stream.
func renderEmbeddedMarginBox(t *testing.T, content string, emb *font.EmbeddedFont) string {
	t.Helper()
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Body", font.Helvetica, 12))
	r.SetMarginBoxes(map[string]MarginBox{
		"bottom-center": {Content: content, FontSize: 9, Embedded: emb},
	})
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least one page")
	}
	return string(pages[0].Stream.Bytes())
}

// TestMarginBoxEmbeddedDrawsRealGlyphs is the regression guard demanded
// by the issue #328 review: a margin box whose Embedded font covers the
// drawn characters must emit a NON-zero GID stream. A regression that
// routed margin-box text to .notdef (GID 0) — or that failed to encode
// the characters at all — would produce an all-zero hex stream and fail
// here.
//
// The drawn content uses CJK runes the synthetic fixture covers (its
// cmap assigns the first checked-in codepoint, 中, to GID 1, etc.), so
// the expected GID stream is provably non-zero. ASCII page numbers
// CANNOT be used with this fixture because it has no Latin coverage; see
// the package comment and the companion html-package test.
func TestMarginBoxEmbeddedDrawsRealGlyphs(t *testing.T) {
	emb := loadSyntheticCJKEmbedded(t)
	// 中华国 are the 1st, 2nd and 7th codepoints in the fixture's
	// codepoint list, so they map to GIDs 1, 2 and 7 respectively.
	const content = "中华国"

	stream := renderEmbeddedMarginBox(t, content, emb)

	// The margin box must use the embedded Identity-H hex path, not the
	// standard-font ShowText path.
	m := hexTjRe.FindStringSubmatch(stream)
	if m == nil {
		t.Fatalf("no <HEX> Tj embedded-text operator in margin-box stream; "+
			"margin box did not draw via the embedded font:\n%s", stream)
	}
	hex := m[1]

	// Each GID is two bytes (four hex chars). An all-zero stream means
	// every glyph resolved to .notdef — exactly the invisible-text
	// regression this test exists to catch.
	if hex == "" || strings.Trim(hex, "0") == "" {
		t.Fatalf("margin-box embedded GID stream is all .notdef (GID 0): <%s>; "+
			"covered characters were routed to .notdef", hex)
	}

	// Cross-check against the font's own encoder: the rendered hex must
	// equal EncodeString of the same content, and that encoding must be
	// non-zero. EncodeString is the exact API the production draw path
	// uses (font.EmbeddedFont.EncodeString via drawWordEmbedded).
	wantHex := strings.ToUpper(hexOf(emb.EncodeString(content)))
	if strings.ToUpper(hex) != wantHex {
		t.Fatalf("margin-box GID stream %q does not match EncodeString(%q)=%q",
			hex, content, wantHex)
	}
}

// TestMarginBoxEmbeddedNotdefWouldFailGuard documents (and proves) that
// the GID-non-zero assertion in TestMarginBoxEmbeddedDrawsRealGlyphs
// genuinely fails when text is routed to .notdef. It feeds the SAME
// embedded font ASCII content, which the CJK-only fixture maps entirely
// to GID 0, and asserts the resulting stream IS all-zero — i.e. the
// failure mode the regression guard detects is real for this font.
func TestMarginBoxEmbeddedNotdefWouldFailGuard(t *testing.T) {
	emb := loadSyntheticCJKEmbedded(t)
	// ASCII has no coverage in the CJK fixture → every rune is .notdef.
	stream := renderEmbeddedMarginBox(t, "Page 1", emb)

	m := hexTjRe.FindStringSubmatch(stream)
	if m == nil {
		t.Fatalf("expected an embedded <HEX> Tj operator even for .notdef text:\n%s", stream)
	}
	if strings.Trim(m[1], "0") != "" {
		t.Fatalf("expected ASCII to map entirely to .notdef (all-zero GIDs) in the "+
			"CJK-only fixture, got non-zero <%s>; the GID-non-zero guard would not "+
			"distinguish real glyphs from .notdef if this changed", m[1])
	}
	// The non-zero guard from the sibling test, applied to this stream,
	// MUST trip — proving that test would catch a .notdef regression.
	allNotdef := strings.Trim(m[1], "0") == ""
	if !allNotdef {
		t.Fatal("inconsistent: .notdef stream not detected as all-zero")
	}
}

// hexOf renders bytes as an uppercase hex string the way ShowTextHex
// does (without the surrounding <> Tj).
func hexOf(b []byte) string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0x0F])
	}
	return string(out)
}
