// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"encoding/binary"
	"os"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// TestFallbackChainResolvesTTCViaBaseFS pins the end-to-end claim from
// the #235 PR description: with TTC dispatch fixed in font.ParseFont,
// a CJK-style paragraph that flows through the fallback chain
// (Options.FallbackFontPath via BaseFS) resolves to the embedded TTC
// face rather than rendering as .notdef glyphs.
//
// This test deliberately uses a relative FallbackFontPath against a
// fstest.MapFS so the routing exercises font.ParseFont (the TTC branch
// landed in this PR) without touching the absolute-path /
// BaseFS-bypass behavior that issue #229 will centralize. A future
// regression in the TTC dispatch — even one that compiled cleanly —
// would manifest here as Embedded == nil because loadFallbackFont
// would fail to parse the TTC bytes.
//
// The synthetic TTC wraps a Latin TTF and therefore does not carry
// CJK glyphs of its own; the test pins that the *fallback font is
// loaded and selected*, not that any specific codepoint resolves to a
// glyph (that is a property of the font the end user installs, not of
// the dispatch code under test).
func TestFallbackChainResolvesTTCViaBaseFS(t *testing.T) {
	ttfBytes := loadAnyHTMLTestTTF(t)
	ttcBytes := wrapTTFAsSyntheticTTC(t, ttfBytes)

	want, err := font.ParseFont(ttcBytes)
	if err != nil {
		t.Fatalf("synthetic TTC failed to parse via font.ParseFont (TTC dispatch broken?): %v", err)
	}
	wantPS := want.PostScriptName()

	fsys := fstest.MapFS{
		"fonts/synthetic.ttc": &fstest.MapFile{Data: ttcBytes},
	}

	// Use a non-WinAnsi codepoint so the renderer is forced into the
	// fallback path. Any character outside WinAnsiEncoding works; we
	// pick a Chinese ideograph because that is the user's reported
	// scenario in #227.
	src := `<html><body><p>中</p></body></html>`

	elems, err := Convert(src, &Options{
		BaseFS:           fsys,
		FallbackFontPath: "fonts/synthetic.ttc",
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("expected paragraph element")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *layout.Paragraph, got %T", elems[0])
	}
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatal("paragraph rendered no words")
	}
	w := lines[0].Words[0]
	if w.Embedded == nil {
		t.Fatal("rendered word has no embedded font; TTC fallback failed to load")
	}
	if got := w.Embedded.Face().PostScriptName(); got != wantPS {
		t.Errorf("fallback PostScriptName = %q, want %q (TTC dispatch may have been bypassed)", got, wantPS)
	}
}

// wrapTTFAsSyntheticTTC mirrors font.buildSyntheticTTC: builds a TTC v1
// envelope that places one face at the given TTF body, with directory
// offsets rewritten to be absolute within the resulting TTC. Kept local
// to the html package because the original is a test helper in the
// font package; duplication is cheap (~25 LOC) and keeps the public
// font API surface unchanged.
func wrapTTFAsSyntheticTTC(t *testing.T, ttfBytes []byte) []byte {
	t.Helper()
	const headerSize = 12 + 4 // 12-byte header + one uint32 face offset
	out := make([]byte, headerSize+len(ttfBytes))
	copy(out[0:4], "ttcf")
	binary.BigEndian.PutUint32(out[4:8], 0x00010000) // version 1.0
	binary.BigEndian.PutUint32(out[8:12], 1)         // numFonts
	binary.BigEndian.PutUint32(out[12:16], headerSize)
	copy(out[headerSize:], ttfBytes)

	if len(out) < headerSize+12 {
		t.Fatalf("ttf bytes too short to wrap (len=%d)", len(ttfBytes))
	}
	numTables := int(binary.BigEndian.Uint16(out[headerSize+4:]))
	for i := range numTables {
		entryBase := headerSize + 12 + i*16
		oldOff := binary.BigEndian.Uint32(out[entryBase+8:])
		binary.BigEndian.PutUint32(out[entryBase+8:], oldOff+uint32(headerSize))
	}
	return out
}

// loadAnyHTMLTestTTF locates any TTF on the host. The candidate list
// mirrors font/ttc_test.go's loadAnySystemTTF so the html-layer test
// can be exercised on the same hosts that exercise the font-layer
// fixture. Skips when no TTF is available; the synthetic TTC cannot be
// built without source bytes.
func loadAnyHTMLTestTTF(t *testing.T) []byte {
	t.Helper()
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/System/Library/Fonts/Supplemental/Courier New.ttf",
			"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
		}
	case "linux":
		candidates = []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		}
	case "windows":
		candidates = []string{
			`C:\Windows\Fonts\arial.ttf`,
			`C:\Windows\Fonts\segoeui.ttf`,
			`C:\Windows\Fonts\tahoma.ttf`,
		}
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return data
		}
	}
	t.Skip("no system TTF found to build a synthetic TTC for fallback chain test")
	return nil
}
