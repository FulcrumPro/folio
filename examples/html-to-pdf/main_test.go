// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestHTMLToPDFExampleProducesValidPDF runs the example's HTML → PDF
// pipeline against the bundled reportHTML constant and asserts the
// resulting bytes are a structurally plausible PDF. The example
// previously had no CI of any kind — the reporter of issue #295 found
// the multi-megabyte CJK blowup by running it on Ubuntu; nothing in
// folio's automated tests would have caught the regression. This is
// the smoke test that closes that gap for this example (issue #231).
func TestHTMLToPDFExampleProducesValidPDF(t *testing.T) {
	doc, err := buildDocument(reportHTML)
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	pdf := buf.Bytes()

	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	// The example renders a fully-styled multi-section report; the
	// output is unconditionally well above 5 KB even with the
	// post-#300 font deduplication. A drop below this floor would
	// mean the conversion bailed out and produced an empty PDF.
	if len(pdf) < 5000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >5 KB", len(pdf))
	}
}

// TestHTMLToPDFExampleTwoPages locks in the page count. The HTML has
// an explicit `.page-break { break-before: page }` div between the
// dashboard and the leadership section; if the page-break CSS handling
// regressed silently, this test would catch it as a single-page output.
func TestHTMLToPDFExampleTwoPages(t *testing.T) {
	doc, err := buildDocument(reportHTML)
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	pdf := buf.String()
	// /Count <N> appears on the /Pages catalog root and lists the
	// total page count for the document.
	if !strings.Contains(pdf, "/Count 2") {
		t.Errorf("expected /Count 2 in /Pages root; example HTML has one page-break and should produce two pages")
	}
}

// TestHTMLToPDFExampleMetadataFromHTML verifies the converter pulls
// /Info Title and Author from the document's <title> and
// <meta name="author"> tags. The example deliberately sets defaults
// on the Document and then lets ConvertFull's Metadata struct
// override them; if that override silently breaks, the PDF's Info
// dictionary would carry the placeholder values instead.
func TestHTMLToPDFExampleMetadataFromHTML(t *testing.T) {
	doc, err := buildDocument(reportHTML)
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	pdf := buf.String()
	if !strings.Contains(pdf, "Q4 2026 Quarterly Report") {
		t.Error("PDF /Info missing title from <title> tag")
	}
	if !strings.Contains(pdf, "Apex Capital Partners") {
		t.Error("PDF /Info missing author from <meta name=\"author\"> tag")
	}
}

// TestHTMLToPDFExampleFontDedupAcrossPages pins the document-level
// font-sharing fix landed in #300 (commit 61dc047). The example styles
// body text as Helvetica and headings as Helvetica-Bold across two
// pages. Pre-fix, each variant appeared once per page (4 total font
// dict entries); post-fix, each variant appears exactly once for the
// whole document. The assertion checks both variants by name to keep
// the signal specific — a generic "/BaseFont count" check would not
// distinguish a per-page regression from a font-variant-count change.
func TestHTMLToPDFExampleFontDedupAcrossPages(t *testing.T) {
	doc, err := buildDocument(reportHTML)
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	pdf := buf.String()
	cases := []string{"/BaseFont /Helvetica ", "/BaseFont /Helvetica-Bold"}
	for _, marker := range cases {
		got := strings.Count(pdf, marker)
		if got != 1 {
			t.Errorf("count of %q = %d, want 1 (post-#300 single-embed-per-document)", marker, got)
		}
	}
}

// TestHTMLToPDFExampleLinkAnnotation verifies that the "Built with
// Folio" anchor in the report footer produces a `/Subtype /Link`
// annotation in the output. A regression that drops link-annotation
// emission (or routes it through a different subtype) would mean the
// example's clickable URL stops working — that's a real user-visible
// degradation that the static rendering tests do not catch.
func TestHTMLToPDFExampleLinkAnnotation(t *testing.T) {
	doc, err := buildDocument(reportHTML)
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	pdf := buf.String()
	if !strings.Contains(pdf, "/Subtype /Link") {
		t.Error("expected at least one /Subtype /Link annotation from the <a href> in the report footer")
	}
}
