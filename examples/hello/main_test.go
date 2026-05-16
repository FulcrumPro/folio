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

// TestHelloExampleProducesValidPDF is the smoke test for folio's
// most-trivial example. The example only emits a heading and one
// paragraph; the size floor is intentionally lower than larger
// examples because there is genuinely less content to produce.
func TestHelloExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1 KB", len(pdf))
	}
}

func TestHelloExampleSinglePage(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 1 {
		t.Errorf("PageCount = %d, want 1", got)
	}
}

func TestHelloExampleMetadataInInfoDict(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "Hello World" {
		t.Errorf("/Info Title = %q, want %q", title, "Hello World")
	}
	if author != "Folio" {
		t.Errorf("/Info Author = %q, want %q", author, "Folio")
	}
}

// TestHelloExampleHeadingAndParagraphContent reads the page-1 content
// stream and asserts the literal text the example emits appears in
// the rendered text. Heading + paragraph are the only two visible
// elements; if either layout primitive silently drops its content,
// this is where it surfaces.
func TestHelloExampleHeadingAndParagraphContent(t *testing.T) {
	r := examplePDFReader(t)
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{"Hello, Folio!", "pure-Go library"} {
		if !strings.Contains(text, want) {
			t.Errorf("page text missing %q; extracted:\n%s", want, text)
		}
	}
}
