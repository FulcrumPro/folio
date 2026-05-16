// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/carlos7ags/folio/reader"
	"github.com/carlos7ags/folio/tmpl"
)

// examplePDFBytes renders the example invoice once and caches the
// result. The template + sampleInvoice + tmpl.RenderDocument pipeline
// is what main() does on disk; the test exercises the same path
// against an in-memory buffer.
var examplePDFBytes = sync.OnceValue(func() []byte {
	doc, err := tmpl.RenderDocument(invoiceTemplate, sampleInvoice(), nil)
	if err != nil {
		panic("RenderDocument: " + err.Error())
	}
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

func TestTemplateExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1500 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1.5 KB", len(pdf))
	}
}

// TestTemplateExampleVariablesSubstituted is the central assertion
// for this example: the {{.Number}}, {{.Customer}}, {{.Items}}
// (via range), and the printf-formatted {{.Total}} placeholders in
// the html/template must produce literal values in the rendered PDF.
// A regression that skips substitution would leave raw "{{...}}"
// markers in the page text — which we explicitly negative-assert on
// — or drop the values entirely, which the positive assertions catch.
func TestTemplateExampleVariablesSubstituted(t *testing.T) {
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
		"Invoice #1042",               // {{.Number}}
		"Globex Corporation",          // {{.Customer}}
		"Consulting (40 hrs)",         // {{range .Items}}
		"$6488.00",                    // {{printf "%.2f" .Total}}
		"Payment due within 30 days.", // {{.Notes}} under {{if .Notes}}
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered text missing %q; html/template substitution may have regressed. Full text:\n%s", want, text)
		}
	}
	// Negative assertion: no raw template markers leaked through.
	for _, leak := range []string{"{{.", "{{range", "{{if"} {
		if strings.Contains(text, leak) {
			t.Errorf("raw template marker %q leaked into rendered PDF — substitution did not run", leak)
		}
	}
}
