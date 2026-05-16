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
// caches the result. The template file resolves via the same
// findTemplate helper main() uses, so `go test ./examples/import-page/`
// (cwd = package dir) and `go test ./...` (cwd = repo root) both work
// — findTemplate's candidate list covers both.
var examplePDFBytes = sync.OnceValue(func() []byte {
	doc, err := buildDocument(findTemplate())
	if err != nil {
		panic("buildDocument: " + err.Error())
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

func TestImportPageExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	// The example imports a 29 KB template three times plus overlay
	// text per page; output is reliably >10 KB even with form-XObject
	// deduplication.
	if len(pdf) < 10000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >10 KB", len(pdf))
	}
}

// TestImportPageExampleThreeReceipts pins the receipt count: the
// example renders one page per receipt entry, and there are three
// entries. A regression in the per-page ImportPage / AddText loop
// (e.g. silently skipping iterations after the first) shows up as a
// page count of 1.
func TestImportPageExampleThreeReceipts(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 3 {
		t.Errorf("PageCount = %d, want 3 (one page per receipt entry)", got)
	}
}

func TestImportPageExampleMetadataTitle(t *testing.T) {
	r := examplePDFReader(t)
	title, _, _, _, _ := r.Info()
	if title != "Payment Receipts" {
		t.Errorf("/Info Title = %q, want %q", title, "Payment Receipts")
	}
}

// TestImportPageExampleOverlayTextPerPage verifies the overlay text
// AddText'd on top of each imported template lands on the correct
// page. Each page is a separate receipt with a unique receipt number,
// payee, and amount; if ImportPage misroutes the overlay
// (e.g. all pages render with the same data) the per-page text
// extraction would show duplicate content.
func TestImportPageExampleOverlayTextPerPage(t *testing.T) {
	r := examplePDFReader(t)
	cases := []struct {
		page int
		want []string
	}{
		{0, []string{"REC-001", "Apex Capital Partners", "$34,948.88"}},
		{1, []string{"REC-002", "Meridian Dynamics", "$12,500.00"}},
		{2, []string{"REC-003", "Northwind Technologies", "$8,750.00"}},
	}
	for _, tc := range cases {
		page, err := r.Page(tc.page)
		if err != nil {
			t.Errorf("Page(%d): %v", tc.page, err)
			continue
		}
		text, err := page.ExtractText()
		if err != nil {
			t.Errorf("page %d ExtractText: %v", tc.page, err)
			continue
		}
		for _, want := range tc.want {
			if !strings.Contains(text, want) {
				t.Errorf("page %d overlay missing %q; ImportPage may be misrouting per-page receipt data. Text:\n%s",
					tc.page, want, text)
			}
		}
	}
}
