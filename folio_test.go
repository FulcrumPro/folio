// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package folio_test

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio"
	"github.com/carlos7ags/folio/document"
)

// TestFacadeRoundTrip exercises the façade end to end: construct via the
// root package, then write a PDF.
func TestFacadeRoundTrip(t *testing.T) {
	doc := folio.NewDocument(folio.PageSizeA4)
	doc.Info = folio.Info{Title: "Façade Test"}

	var buf bytes.Buffer
	n, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo via façade: %v", err)
	}
	if n == 0 || buf.Len() == 0 {
		t.Fatal("expected non-empty PDF output")
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("output is not a PDF")
	}
}

// TestFacadeAliasesAreIdentical confirms the re-exports are aliases (not
// distinct types), so values cross the façade/document boundary freely. The
// cross-package function calls below compile only if each pair of names
// denotes the same type.
func TestFacadeAliasesAreIdentical(t *testing.T) {
	// A *document.Document satisfies a *folio.Document parameter with no
	// conversion, and a folio.PageSize satisfies a document.PageSize one.
	acceptFolioDoc := func(*folio.Document) {}
	acceptFolioDoc(document.NewDocument(document.PageSizeLetter))

	acceptDocPageSize := func(document.PageSize) {}
	acceptDocPageSize(folio.PageSizeLegal)

	if folio.PageSizeA4 != document.PageSizeA4 {
		t.Error("PageSizeA4 re-export diverged from document.PageSizeA4")
	}
}
