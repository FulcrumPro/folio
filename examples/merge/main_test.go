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
	pdf, err := buildMergedPDF()
	if err != nil {
		panic("buildMergedPDF: " + err.Error())
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

func TestMergeExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 2000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >2 KB", len(pdf))
	}
}

// TestMergeExampleCombinedPageCount pins the central behavior the
// reader.Merge call is supposed to demonstrate: a cover (1 page) +
// two single-page reports merged yields a 3-page document. A regression
// where Merge silently drops the first or last source would surface
// as a 1- or 2-page result.
func TestMergeExampleCombinedPageCount(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 3 {
		t.Errorf("PageCount = %d, want 3 (cover + Q3 + Q4)", got)
	}
}

func TestMergeExampleMetadataAppliedAfterMerge(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "Annual Summary 2026" {
		t.Errorf("/Info Title = %q, want %q (set via Modifier.SetInfo after Merge)", title, "Annual Summary 2026")
	}
	if author != "Apex Capital Partners" {
		t.Errorf("/Info Author = %q, want %q", author, "Apex Capital Partners")
	}
}

// TestMergeExamplePagesPreserveSourceText verifies the merged PDF
// retained the distinguishing text from each source document on the
// expected page index. The merge order in the example is
// (cover, Q3 report, Q4 report); a regression that mis-orders the
// merge or drops source content would land here.
func TestMergeExamplePagesPreserveSourceText(t *testing.T) {
	r := examplePDFReader(t)
	cases := []struct {
		page int
		want string
	}{
		{0, "Annual Summary 2026"},            // cover
		{1, "Q3 2026 Revenue Report"},         // first report (page index 1 after the cover)
		{1, "Total revenue reached $25.1M"},   // Q3 body text
		{2, "Q4 2026 Revenue Report"},         // second report
		{2, "Asia-Pacific region grew 31.7%"}, // Q4 body text
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
		if !strings.Contains(text, tc.want) {
			t.Errorf("page %d text missing %q; merge may have re-ordered or lost source content. Extracted:\n%s",
				tc.page, tc.want, text)
		}
	}
}
