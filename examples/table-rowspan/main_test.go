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

func TestRowspanExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1 KB", len(pdf))
	}
}

func TestRowspanExampleSinglePage(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 1 {
		t.Errorf("PageCount = %d, want 1", got)
	}
}

// TestRowspanExampleContent confirms every cell — including the cells
// in rows that sit beside a rowspanning cell — is emitted. Before the
// rowspan geometry fix the spanning cells still rendered; the failure
// mode was purely visual (drawn one row tall), so this guards content
// rather than geometry. The geometry assertions live in the layout
// package's TestTableRowspan* tests.
func TestRowspanExampleContent(t *testing.T) {
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
		"Span", "B1", "B2",
		"Morning", "Registration", "Keynote", "Workshop intro", "All day: help desk",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("page text missing %q; extracted:\n%s", want, text)
		}
	}
}
