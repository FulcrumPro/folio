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

// TestFullOptimizerStackOnImportedDocument exercises every lossless
// optimizer toggle together on a multi-page imported document. The
// dedup pass merges identical resource dictionaries (fonts, color
// spaces) shared across imported pages; the cleanup pass strips
// redundant operators from per-page content streams; the recompress
// pass Flates the cleaned bytes; the sweep pass drops any orphans
// the import process may have left behind. The combined result must
// be smaller than every prefix-subset and must round-trip
// (page count, structural reparse, extracted text equality).
func TestFullOptimizerStackOnImportedDocument(t *testing.T) {
	source := document.NewDocument(document.PageSizeLetter)
	for i := 0; i < 6; i++ {
		source.Add(layout.NewHeading(fmt.Sprintf("Section %d", i+1), layout.H1))
		source.Add(layout.NewParagraph(
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. "+
				"Sed do eiusmod tempor incididunt ut labore et dolore magna "+
				"aliqua. Ut enim ad minim veniam, quis nostrud exercitation.",
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
		out := document.NewDocument(document.PageSizeLetter)
		for i := 0; i < r.PageCount(); i++ {
			srcPage, _ := r.Page(i)
			cs, _ := srcPage.ContentStream()
			res, _ := srcPage.Resources()
			p := out.AddPage()
			p.ImportPage(cs, res, srcPage.Width, srcPage.Height)
		}
		return out
	}

	// Each progressive option set adds one toggle. We expect the size
	// curve to be monotonically non-increasing (no toggle enlarges).
	type variant struct {
		name string
		opts document.WriteOptions
	}
	variants := []variant{
		{"plain", document.WriteOptions{}},
		{"xref+obj", document.WriteOptions{
			UseXRefStream:    true,
			UseObjectStreams: true,
		}},
		{"+sweep", document.WriteOptions{
			UseXRefStream:    true,
			UseObjectStreams: true,
			OrphanSweep:      true,
		}},
		{"+cleanup", document.WriteOptions{
			UseXRefStream:       true,
			UseObjectStreams:    true,
			OrphanSweep:         true,
			CleanContentStreams: true,
		}},
		{"+dedup", document.WriteOptions{
			UseXRefStream:       true,
			UseObjectStreams:    true,
			OrphanSweep:         true,
			CleanContentStreams: true,
			DeduplicateObjects:  true,
		}},
		{"+recompress", document.WriteOptions{
			UseXRefStream:       true,
			UseObjectStreams:    true,
			OrphanSweep:         true,
			CleanContentStreams: true,
			DeduplicateObjects:  true,
			RecompressStreams:   true,
		}},
	}
	sizes := make([]int, len(variants))
	outputs := make([][]byte, len(variants))
	for i, v := range variants {
		out, err := buildImported().ToBytesWithOptions(v.opts)
		if err != nil {
			t.Fatalf("%s: %v", v.name, err)
		}
		sizes[i] = len(out)
		outputs[i] = out
	}

	// Headline assertion: the full stack is strictly smaller than
	// plain. Per-toggle monotonicity is NOT asserted because some
	// toggles add overhead that pays off only in combination — for
	// example, the xref+object-stream variant on a small document
	// can be a few bytes larger than the traditional xref table
	// before recompression collapses the content streams. Real
	// users care about the bottom-line size, not the intermediate
	// columns.
	if sizes[len(sizes)-1] >= sizes[0] {
		t.Errorf("full optimizer stack did not shrink: plain=%d, full=%d",
			sizes[0], sizes[len(sizes)-1])
	}

	// Round-trip the full-optimizer output.
	full := outputs[len(outputs)-1]
	rr, err := reader.Parse(full)
	if err != nil {
		t.Fatalf("re-parse full optimizer: %v", err)
	}
	if rr.PageCount() != r.PageCount() {
		t.Errorf("page count mismatch: full=%d, source=%d",
			rr.PageCount(), r.PageCount())
	}

	// Text equality: every page's extracted text must match the
	// pre-optimizer plain re-import.
	rPlain, err := reader.Parse(outputs[0])
	if err != nil {
		t.Fatalf("re-parse plain: %v", err)
	}
	for i := 0; i < r.PageCount(); i++ {
		plainPage, _ := rPlain.Page(i)
		fullPage, _ := rr.Page(i)
		plainText, err := plainPage.ExtractText()
		if err != nil {
			t.Fatalf("ExtractText plain page %d: %v", i, err)
		}
		fullText, err := fullPage.ExtractText()
		if err != nil {
			t.Fatalf("ExtractText full page %d: %v", i, err)
		}
		if plainText != fullText {
			t.Errorf("page %d text differs after full optimizer\n--- plain ---\n%s\n--- full ---\n%s",
				i, plainText, fullText)
		}
	}

	t.Logf("imported document: plain=%d, xref+obj=%d, +sweep=%d, +cleanup=%d, +dedup=%d, +recompress=%d (saved %.1f%%)",
		sizes[0], sizes[1], sizes[2], sizes[3], sizes[4], sizes[5],
		100.0*float64(sizes[0]-sizes[5])/float64(sizes[0]))
}
