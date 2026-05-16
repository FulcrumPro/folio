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

func TestLinksExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 5000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >5 KB", len(pdf))
	}
}

func TestLinksExampleTwoPages(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 2 {
		t.Errorf("PageCount = %d, want 2 (HTML page + layout API page)", got)
	}
}

func TestLinksExampleMetadataInInfoDict(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "Folio Links Showcase" {
		t.Errorf("/Info Title = %q, want %q", title, "Folio Links Showcase")
	}
	if author != "Folio" {
		t.Errorf("/Info Author = %q, want %q", author, "Folio")
	}
}

// TestLinksExampleEmitsLinkAnnotations is the central regression check
// for this example. The HTML and the layout-API pages together
// declare ~15 different link destinations; a count well above 5 is
// the floor that distinguishes "links rendered" from "links silently
// dropped." Pinning a specific number is too brittle because future
// example tweaks may add or remove links, but the floor catches the
// catastrophic failure mode that this example is designed to guard.
func TestLinksExampleEmitsLinkAnnotations(t *testing.T) {
	pdf := string(examplePDFBytes())
	got := strings.Count(pdf, "/Subtype /Link")
	if got < 5 {
		t.Errorf("/Subtype /Link count = %d, want >=5 (HTML + layout-API links)", got)
	}
}

// TestLinksExampleHTMLLinkTargetsPreserved asserts a specific external
// URL from the HTML survives end-to-end through the PDF /Link
// /A /URI action. The example deliberately exercises external
// hyperlinks via <a href>; this catches a regression where the URI
// action is dropped or hardcoded to about:blank, which a
// presence-only annotation count cannot detect.
func TestLinksExampleHTMLLinkTargetsPreserved(t *testing.T) {
	pdf := string(examplePDFBytes())
	for _, want := range []string{
		"https://github.com/carlos7ags/folio",
		"https://pkg.go.dev",
	} {
		if !strings.Contains(pdf, want) {
			t.Errorf("link URI %q not present in PDF; HTML <a href> may have lost its destination through layout/serialization", want)
		}
	}
}
