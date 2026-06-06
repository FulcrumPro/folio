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
	doc, err := buildDocument()
	if err != nil {
		panic("buildDocument: " + err.Error())
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		panic("WriteTo: " + err.Error())
	}
	return buf.Bytes()
})

func TestListStylingExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1 KB", len(pdf))
	}
}

// TestListStylingExampleContent reads the page text and asserts the
// step content (block children) and badge digits all rendered — if the
// <li> layout regressed to flattening or dropping content, this catches it.
func TestListStylingExampleContent(t *testing.T) {
	r, err := reader.Parse(examplePDFBytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{
		"Prepare the document.", "Gather the source files",
		"Confirm the page size", "rounded corners", "1", "2", "3",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("page text missing %q; extracted:\n%s", want, text)
		}
	}
}
