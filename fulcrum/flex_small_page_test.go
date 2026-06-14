// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio/document"
	foliotmpl "github.com/carlos7ags/folio/tmpl"
)

// TestFlexOverflowOnSmallPageTerminates pins the v0.9.1-fulcrum.17 fix: a flex
// whose content is taller than a small fixed-size page must render in a bounded
// number of pages (clipped via force-render), not paginate unboundedly.
//
// fulcrum.14 taught a flex to report it had not fully fit so a no-break wrapper
// could relocate the totals block. But it returned LayoutPartial, which the
// renderer fragments page-by-page. For content taller than a whole page (the
// fixed-size item labels: 2.625in x 1in) that fragmentation never terminated —
// each tiny page fit nothing new, and the parent Div masked the flex's
// LayoutNothing as LayoutPartial-with-empty-box, so the renderer emitted an
// unbounded run of empty pages, each an expensive full re-layout (folio hung
// for minutes rendering the ItemLabel* templates).
//
// The flex now returns LayoutNothing when an item paginates (folio's flex is
// atomic), and a Div that places nothing returns LayoutNothing rather than an
// empty LayoutPartial — so the renderer relocates the block to a fresh page
// and force-renders (clips) it when it is alone at the top and still too tall.
func TestFlexOverflowOnSmallPageTerminates(t *testing.T) {
	// 2.625in x 1in label page (189pt x 72pt), content far taller than 72pt,
	// laid out with the column/row flex nesting the label templates use.
	src := `<html><head><style>
		@page{size:189pt 72pt;margin:0}
		body{margin:0;padding:0}
		.col{display:flex;flex-direction:column}
		.row{display:flex;flex-grow:1}
		.cell{height:40pt}
	</style></head><body>
		<div class="col">
			<div class="row"><div class="cell">ROW-ONE-of-the-label</div></div>
			<div class="row"><div class="cell">ROW-TWO-of-the-label</div></div>
			<div class="row"><div class="cell">ROW-THREE-of-the-label</div></div>
			<div class="row"><div class="cell">ROW-FOUR-of-the-label</div></div>
		</div>
	</body></html>`

	doc, err := foliotmpl.RenderDocument(src, nil, &foliotmpl.Options{
		PageSize: document.PageSize{Width: 189, Height: 72},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	pages := pageCount(buf.Bytes())
	t.Logf("pages=%d", pages)
	// Pre-fix this rendered hundreds/thousands of (empty) pages or hung. A
	// clipped label is a small bounded number of pages.
	if pages > 5 {
		t.Errorf("label overflow produced %d pages — flex paginated unboundedly instead of clipping", pages)
	}
}
