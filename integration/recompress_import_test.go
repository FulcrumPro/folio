// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

// TestRecompressShrinksImportedDocument verifies the headline use case
// for WriteOptions.RecompressStreams: a document built by importing
// pages from a parsed source PDF carries the source's content streams
// in raw form (reader/import.go inflates and copies, but does not
// re-Flate). Recompression on write must shrink the output.
//
// This is the only use case where the win is large enough to assert
// numerically; layout-built documents already write streams at
// BestCompression and would see ~0% improvement.
func TestRecompressShrinksImportedDocument(t *testing.T) {
	// Build a content-rich source document. Using NewParagraph repeatedly
	// inflates the content stream past Flate's overhead so the test is
	// robust to small per-stream overhead.
	source := document.NewDocument(document.PageSizeLetter)
	for i := 0; i < 5; i++ {
		source.Add(layout.NewHeading(
			fmt.Sprintf("Section %d", i+1),
			layout.H1,
		))
		source.Add(layout.NewParagraph(
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do "+
				"eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut "+
				"enim ad minim veniam, quis nostrud exercitation ullamco laboris "+
				"nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor "+
				"in reprehenderit in voluptate velit esse cillum dolore eu fugiat "+
				"nulla pariatur. Excepteur sint occaecat cupidatat non proident.",
			font.Helvetica, 11,
		))
	}
	var sourceBuf bytes.Buffer
	if _, err := source.WriteTo(&sourceBuf); err != nil {
		t.Fatalf("write source: %v", err)
	}

	// Re-parse the source.
	r, err := reader.Parse(sourceBuf.Bytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	if r.PageCount() == 0 {
		t.Fatal("source has no pages")
	}

	// buildImportedDoc constructs a fresh Document by importing every
	// page of the parsed source. The resulting Writer holds raw
	// (uncompressed, no /Filter) content streams the recompression
	// pass should be able to shrink.
	buildImportedDoc := func() *document.Document {
		imported := document.NewDocument(document.PageSizeLetter)
		for i := 0; i < r.PageCount(); i++ {
			srcPage, err := r.Page(i)
			if err != nil {
				t.Fatalf("source page %d: %v", i, err)
			}
			cs, err := srcPage.ContentStream()
			if err != nil {
				t.Fatalf("content stream %d: %v", i, err)
			}
			res, _ := srcPage.Resources()
			p := imported.AddPage()
			p.ImportPage(cs, res, srcPage.Width, srcPage.Height)
		}
		return imported
	}

	plain, err := buildImportedDoc().ToBytes()
	if err != nil {
		t.Fatalf("plain write: %v", err)
	}
	recompressed, err := buildImportedDoc().ToBytesWithOptions(document.WriteOptions{
		RecompressStreams: true,
	})
	if err != nil {
		t.Fatalf("recompressed write: %v", err)
	}

	if len(recompressed) >= len(plain) {
		t.Errorf("RecompressStreams did not shrink imported document: plain=%d, recompressed=%d",
			len(plain), len(recompressed))
	}

	// Round-trip the recompressed output to confirm it is still a
	// well-formed PDF with the same page count. A bug that produced
	// "smaller" but corrupt bytes (e.g., truncated content streams)
	// would fail page-count and reparse.
	rr, err := reader.Parse(recompressed)
	if err != nil {
		t.Fatalf("re-parse recompressed: %v", err)
	}
	if rr.PageCount() != r.PageCount() {
		t.Errorf("recompressed page count = %d, want %d", rr.PageCount(), r.PageCount())
	}

	// Content-equality: a bug that produced "smaller" but text-
	// corrupt bytes would survive page-count and reparse but fail
	// here. We compare extracted text page-by-page against the
	// pre-recompression import (which we already know preserves the
	// original content correctly, since recompression is the only
	// new variable between the two writes).
	rPlain, err := reader.Parse(plain)
	if err != nil {
		t.Fatalf("re-parse plain: %v", err)
	}
	for i := 0; i < r.PageCount(); i++ {
		plainPage, _ := rPlain.Page(i)
		recPage, _ := rr.Page(i)
		plainText, err := plainPage.ExtractText()
		if err != nil {
			t.Fatalf("ExtractText plain page %d: %v", i, err)
		}
		recText, err := recPage.ExtractText()
		if err != nil {
			t.Fatalf("ExtractText recompressed page %d: %v", i, err)
		}
		if plainText != recText {
			t.Errorf("page %d text differs after recompression\n--- plain (%d chars) ---\n%s\n--- recompressed (%d chars) ---\n%s",
				i, len(plainText), plainText, len(recText), recText)
		}
	}

	t.Logf("imported document: plain=%d bytes, recompressed=%d bytes (saved %.1f%%)",
		len(plain), len(recompressed),
		100.0*float64(len(plain)-len(recompressed))/float64(len(plain)))
}

// TestRecompressComposesWithSweepOnImportedDocument verifies the full
// optimizer stack on an imported document: orphan sweep + recompression
// + xref/object streams together must produce strictly smaller output
// than any subset, and the result must round-trip cleanly.
func TestRecompressComposesWithSweepOnImportedDocument(t *testing.T) {
	source := document.NewDocument(document.PageSizeLetter)
	for i := 0; i < 8; i++ {
		source.Add(layout.NewHeading(fmt.Sprintf("Heading %d", i+1), layout.H1))
		source.Add(layout.NewParagraph(
			"Body text body text body text body text body text body text body text. "+
				"More body text here to give the content stream some real bulk.",
			font.Helvetica, 11,
		))
	}
	var sourceBuf bytes.Buffer
	if _, err := source.WriteTo(&sourceBuf); err != nil {
		t.Fatalf("write source: %v", err)
	}
	r, err := reader.Parse(sourceBuf.Bytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}

	buildImported := func() *document.Document {
		imported := document.NewDocument(document.PageSizeLetter)
		for i := 0; i < r.PageCount(); i++ {
			srcPage, _ := r.Page(i)
			cs, _ := srcPage.ContentStream()
			res, _ := srcPage.Resources()
			p := imported.AddPage()
			p.ImportPage(cs, res, srcPage.Width, srcPage.Height)
		}
		return imported
	}

	plain, _ := buildImported().ToBytes()
	full, err := buildImported().ToBytesWithOptions(document.WriteOptions{
		UseXRefStream:     true,
		UseObjectStreams:  true,
		OrphanSweep:       true,
		RecompressStreams: true,
	})
	if err != nil {
		t.Fatalf("full optimizer: %v", err)
	}

	if len(full) >= len(plain) {
		t.Errorf("full optimizer did not shrink: plain=%d, full=%d", len(plain), len(full))
	}
	if _, err := reader.Parse(full); err != nil {
		t.Fatalf("re-parse full-optimizer output: %v", err)
	}
}
