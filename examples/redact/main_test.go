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

// examplePDFBytes runs the example's redaction pipeline once and
// caches the resulting redacted bytes. Mirrors main()'s final write
// (text + SSN-regex redaction with metadata stripping).
var examplePDFBytes = sync.OnceValue(func() []byte {
	pdf, err := buildRedactedPDF()
	if err != nil {
		panic("buildRedactedPDF: " + err.Error())
	}
	return pdf
})

func examplePDFReader(t *testing.T) *reader.PdfReader {
	t.Helper()
	r, err := reader.Parse(examplePDFBytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r
}

func TestRedactExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1 KB", len(pdf))
	}
}

func TestRedactExamplePreservesPageCount(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 1 {
		t.Errorf("PageCount = %d, want 1 (source has one page)", got)
	}
}

// TestRedactExampleSSNNotInRawBytes is the central security guarantee
// of this example: a regex-matched SSN must NOT appear anywhere in
// the serialized PDF bytes after redaction. main() itself runs this
// exact check at the tail of the redaction pipeline.
//
// Sanity precondition: extract the unredacted source's page text
// (which decompresses the content stream) and confirm the SSN is
// present there. Without this, a regression in createSensitivePDF
// that drops the SSN string would let this test silently green. We
// cannot use bytes.Contains on the raw source bytes for the
// precondition because content streams are FlateDecode-compressed,
// so the literal "987-65-4321" only appears in the decompressed
// view.
func TestRedactExampleSSNNotInRawBytes(t *testing.T) {
	pdf := examplePDFBytes()
	const ssn = "987-65-4321"

	sourceR, err := reader.Parse(createSensitivePDF())
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	srcPage, err := sourceR.Page(0)
	if err != nil {
		t.Fatalf("source Page(0): %v", err)
	}
	srcText, err := srcPage.ExtractText()
	if err != nil {
		t.Fatalf("source ExtractText: %v", err)
	}
	if !strings.Contains(srcText, ssn) {
		t.Fatalf("createSensitivePDF lost its SSN literal; test no longer exercises the redactor")
	}

	if bytes.Contains(pdf, []byte(ssn)) {
		t.Errorf("redacted PDF still contains SSN %q in raw bytes; reader.RedactPattern leaked the redacted glyph run", ssn)
	}
}

// TestRedactExampleSSNNotInExtractedText asserts the SSN is gone from
// the page's extractable text layer too. Raw-bytes absence is the
// stronger property; extracted-text absence is a sanity check that
// pdftotext-style consumers see the redaction.
func TestRedactExampleSSNNotInExtractedText(t *testing.T) {
	r := examplePDFReader(t)
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if strings.Contains(text, "987-65-4321") {
		t.Errorf("extracted text still contains SSN; redaction did not remove the visible glyph run.\nText:\n%s", text)
	}
}

// TestRedactExampleOverlayTextReplacesSSN verifies the OverlayText
// option actually painted "[SSN]" over the redacted region. The
// pre-redaction text does not contain "[SSN]"; after redaction with
// OverlayText: "[SSN]", the page text should.
func TestRedactExampleOverlayTextReplacesSSN(t *testing.T) {
	r := examplePDFReader(t)
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "[SSN]") {
		t.Errorf("extracted text missing overlay marker %q; reader.RedactPattern OverlayText option may have regressed.\nText:\n%s", "[SSN]", text)
	}
}

// TestRedactExampleMetadataStripped pins the StripMetadata option:
// after redaction, /Info Title and Author must be empty strings (or
// missing entries that read as empty). createSensitivePDF sets both
// "Employee Record" and "HR Department"; if StripMetadata regresses,
// those leak through.
func TestRedactExampleMetadataStripped(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "" {
		t.Errorf("/Info Title = %q after StripMetadata; expected empty", title)
	}
	if author != "" {
		t.Errorf("/Info Author = %q after StripMetadata; expected empty", author)
	}
}
