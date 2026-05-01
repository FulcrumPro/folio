// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/reader"
)

// TestCJKDropSfntRoundTripsThroughPDFExtraction is the end-to-end
// pin for the user-facing correctness claim of issue #260: dropping
// sfnt from the metric path doesn't silently break the embedded
// font's /ToUnicode CMap. A regression where shaping correctly
// produces glyph IDs (so the rendered PDF looks visually correct)
// but the ToUnicode CMap is built against the wrong source would
// pass every other test in this package — accessibility tools,
// copy-paste, and PDF search would all be silently broken.
//
// The test mirrors the original #227 reporter's scenario: render
// Chinese text through the example flow with a CJK TTC that
// previously failed to load (sfnt's maxCmapSegments=20000 limit
// blocked it), parse the resulting PDF back, extract text via the
// same path Acrobat / pdftotext / screen readers use, and assert
// the original Chinese phrase round-trips byte-perfect.
//
// macOS-only because STHeiti is the universally-available
// recovery-path-triggering CJK font on dev hosts. Linux/Windows
// CI exercises the wiring synthetically via
// font/cmap_test.go::TestParseCmapFormat12LargeGroupCount; this
// test adds the integration-level user-facing claim on the platform
// where the real fixture exists.
func TestCJKDropSfntRoundTripsThroughPDFExtraction(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("STHeiti is the universally-available large-CJK TTC fixture; only on darwin")
	}
	const stHeiti = "/System/Library/Fonts/STHeiti Light.ttc"
	if _, err := os.Stat(stHeiti); err != nil {
		t.Skipf("STHeiti not present: %v", err)
	}
	const want = "中华人民共和国是一个历史悠久的文明古国"

	htmlStr := fmt.Sprintf(`<!DOCTYPE html>
<html><head><style>
@font-face { font-family: 'CJK'; src: url('%s'); }
body { font-family: 'CJK'; font-size: 14px; }
</style></head><body><p>%s。</p></body></html>`, stHeiti, want)

	result, err := html.ConvertFull(htmlStr, &html.Options{StrictAssets: true})
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	doc := document.NewDocument(document.PageSizeLetter)
	for _, e := range result.Elements {
		doc.Add(e)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	// Re-parse the PDF and extract text from page 0. This walks the
	// embedded subset font's /ToUnicode CMap to convert glyph IDs back
	// to codepoints — the same path Acrobat / pdftotext / screen
	// readers take.
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	if r.PageCount() == 0 {
		t.Fatal("PDF has no pages")
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	got, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !bytes.Contains([]byte(got), []byte(want)) {
		t.Errorf("extracted text does not contain the original Chinese; ToUnicode CMap is broken.\n  want substring: %q\n  got: %q", want, got)
	}
}
