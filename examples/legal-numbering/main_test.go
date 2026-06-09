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

func TestLegalNumberingExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1 KB", len(pdf))
	}
}

// TestLegalNumberingExampleContent reads the page text and asserts the
// multi-level ordinals produced by counters()/::marker (1., 1.1., 1.1.1., 2.)
// and representative clause body text are present — proving the native
// nested numbering rendered and no clause content was dropped.
func TestLegalNumberingExampleContent(t *testing.T) {
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
	// counters() joins the whole stack: top-level "1.", nested "1.1.", "1.2.",
	// and the third level "1.2.1" (extracted as "1.2. 1." because the marker
	// tokenizes around its trailing dot-space). "2." and "3." prove the
	// outer increment fired across clauses.
	for _, want := range []string{
		"1.", "1.1.", "1.2.", "1.2. 1.", "2.", "2.1.", "3.",
		"Definitions.", "Master Services Agreement",
		"Term and Termination.", "Confidentiality.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("page text missing %q; extracted:\n%s", want, text)
		}
	}
}
