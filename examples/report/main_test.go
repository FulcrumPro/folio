// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// examplePDFBytes runs the example pipeline once per test process and
// caches the result. Tests share the same byte slice, so the costly
// layout pass runs a single time even though the file declares
// several tests.
var examplePDFBytes = sync.OnceValue(func() []byte {
	doc := buildDocument()
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		panic("WriteTo: " + err.Error())
	}
	return buf.Bytes()
})

func examplePDFReader(t *testing.T) *reader.PdfReader {
	t.Helper()
	r, err := reader.Parse(examplePDFBytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r
}

// TestReportExampleProducesValidPDF asserts the example produces a
// well-formed PDF of plausible size. The example builds four full
// pages of content directly via the layout API (no HTML); the size
// floor catches the regression where a layout primitive silently
// short-circuits and produces an under-populated PDF.
func TestReportExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 5000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >5 KB", len(pdf))
	}
}

// TestReportExampleFourPages locks in the page count. The example
// renders a cover page plus three content pages, separated by
// explicit AreaBreak inserts. A regression that breaks the AreaBreak
// layout primitive would show up here as the wrong page count.
func TestReportExampleFourPages(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 4 {
		t.Errorf("PageCount = %d, want 4 (cover + 3 content pages)", got)
	}
}

// TestReportExampleMetadataInInfoDict asserts /Info Title and Author
// are set from the values main() configures on the Document. Routed
// through reader.Info() so the assertion inspects the actual /Info
// dictionary rather than the serialized bytes — the body text on
// page 3 includes "Apex Capital Partners" and an /Author-dropped
// regression would slip past a substring check.
func TestReportExampleMetadataInInfoDict(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "Annual Report 2026" {
		t.Errorf("/Info Title = %q, want %q", title, "Annual Report 2026")
	}
	if author != "Apex Capital Partners" {
		t.Errorf("/Info Author = %q, want %q", author, "Apex Capital Partners")
	}
}

// TestReportExampleFontDedupAcrossPages stress-tests the
// document-level font-sharing fix from #300 commit 61dc047. This
// example uses Helvetica and Helvetica-Bold across all four pages —
// pre-fix, that would have produced 4 entries of each variant in the
// page-resource font dicts. Post-fix, each variant must appear
// exactly once for the whole document. Catches a regression that
// re-introduces per-page font duplication; failure here would also
// signal that user PDFs grow linearly with page count.
func TestReportExampleFontDedupAcrossPages(t *testing.T) {
	pdf := string(examplePDFBytes())
	for _, marker := range []string{"/BaseFont /Helvetica ", "/BaseFont /Helvetica-Bold"} {
		got := strings.Count(pdf, marker)
		if got != 1 {
			t.Errorf("count of %q = %d, want 1 (post-#300 single-embed-per-document across 4 pages)", marker, got)
		}
	}
}

// TestReportExampleHeaderFooterRenderPageNumbers verifies the
// SetFooterElement page-number string lands on a content page. The
// footer callback returns nil on page 0 (the cover) and renders
// "Page N of 4" on pages 2-4. The check walks the page-2 content
// stream (decompressed) so the assertion sees the actual rendered
// text rather than the raw FlateDecode bytes of the PDF.
func TestReportExampleHeaderFooterRenderPageNumbers(t *testing.T) {
	r := examplePDFReader(t)
	page, err := r.Page(1) // page index 1 = the second page, where the footer first renders
	if err != nil {
		t.Fatalf("Page(1): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "Page 2 of 4") {
		t.Errorf("page 2 text does not contain footer page number %q; SetFooterElement / PageContext.PageIndex / TotalPages plumbing may have regressed. Extracted text:\n%s",
			"Page 2 of 4", text)
	}
}

// TestReportExampleHasOutlines asserts SetAutoBookmarks(true)
// produces a /Type /Outlines tree in the catalog. The example
// renders headings via layout.NewHeading; each becomes a bookmark
// node. A regression that drops auto-bookmark emission would lose
// the document's navigation pane.
func TestReportExampleHasOutlines(t *testing.T) {
	pdf := string(examplePDFBytes())
	if !strings.Contains(pdf, "/Type /Outlines") {
		t.Error("no /Type /Outlines object; SetAutoBookmarks(true) appears not to have emitted the outline tree")
	}
}
