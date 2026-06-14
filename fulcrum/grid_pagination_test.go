// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestGridRowsFlowAcrossPages pins the v0.9.1-fulcrum.11 patch: a run of
// separate `display:grid` blocks in normal block flow must flow across page
// boundaries, not silently drop every block after the first page.
//
// Symptom in production: the .NET v3 invoice / purchase-order / quote /
// sales-order templates render every line-item row as its own `display:grid`
// element (`.v2-items-row { display: grid }` in styles-v3). A multi-page
// document rendered through folio kept only the rows that fit on page 1 and
// dropped the rest — an invoice with 16 line items showed 5.
//
// Root cause: grid.buildOverflowResult's "not even the first row fits" branch
// returned LayoutFull — force-drawing the grid past the page edge and telling
// the parent block it fit — so the parent kept placing subsequent grids off
// the page bottom, where they were clipped. (Flex already returns
// LayoutNothing in the same situation, which is why flex-based content
// paginated correctly.)
//
// Patch: return LayoutNothing there, so the renderer moves the whole grid to
// the next page (and force-renders only when a grid is alone at the top of a
// fresh page, which bounds the recursion).
func TestGridRowsFlowAcrossPages(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<html><head><style>
		body { margin: 0; padding: 20pt; }
		.r { display: grid; grid-template-columns: 40pt 1fr 80pt; border-bottom: 1px solid #ccc; padding: 6pt; font-size: 12px; }
	</style></head><body>`)
	const n = 40
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `<div class="r"><span>%d</span><span>gridrow-%03d</span><span>$%d.00</span></div>`, i, i, i)
	}
	b.WriteString(`</body></html>`)

	pdf, err := renderHTMLToPDF(b.String())
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// 40 grid rows far exceed one page. Pre-patch they were crammed onto a
	// single page (and the overflow clipped).
	if pages := pageCount(pdf); pages < 2 {
		t.Fatalf("expected >=2 pages for %d display:grid rows (rows were dropped onto one page), got %d", n, pages)
	}

	// A late row must still be present somewhere in the output — proves the
	// overflow rows flowed to later pages instead of being dropped.
	if findTextY(pdf, "gridrow-040") < 0 {
		t.Error("last grid row 'gridrow-040' missing from output — overflow grid rows were dropped")
	}
}

// pageCount returns the document's page count from the page-tree /Count entry.
func pageCount(pdf []byte) int {
	m := regexp.MustCompile(`/Count\s+(\d+)`).FindSubmatch(pdf)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(string(m[1]))
	return n
}
