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

var examplePDFBytes = sync.OnceValue(func() []byte {
	var buf bytes.Buffer
	if _, err := buildDocument().WriteTo(&buf); err != nil {
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

func TestBarcodesExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1 KB", len(pdf))
	}
}

func TestBarcodesExampleSinglePage(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 1 {
		t.Errorf("PageCount = %d, want 1", got)
	}
}

func TestBarcodesExampleMetadata(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "Barcodes" {
		t.Errorf("/Info Title = %q, want %q", title, "Barcodes")
	}
	if author != "Folio" {
		t.Errorf("/Info Author = %q, want %q", author, "Folio")
	}
}

// TestBarcodesExampleContent asserts the headings and payload captions the
// example emits appear in the rendered page text.
func TestBarcodesExampleContent(t *testing.T) {
	r := examplePDFReader(t)
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{
		"Barcodes",
		"QR Code",
		"Code 128",
		"EAN-13",
		qrPayload,
		code128Data,
		ean13Data,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("page text missing %q; extracted:\n%s", want, text)
		}
	}
}
