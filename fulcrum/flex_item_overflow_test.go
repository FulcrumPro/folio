// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"fmt"
	"strings"
	"testing"
)

// TestFlexItemOverflowNotDropped pins the v0.9.1-fulcrum.14 patch: when a flex
// container's item is itself fragmentable (a block child that returns
// LayoutPartial because only part of its content fits the remaining page), the
// flex must carry the remainder into its overflow instead of dropping it.
//
// Before the patch, Flex.planRow read only each item plan's Consumed height and
// discarded its Status/Overflow, then reported LayoutFull even though content
// had spilled past the page. The remainder vanished and no further page was
// produced.
//
// The v3 commerce templates wrap the totals in
//
//	<div class="document-summary-wrapper no-break">          (page-break-inside:avoid)
//	  <div class="document-summary-main" style="display:flex">
//	    <div class="note-section" style="flex:2"></div>
//	    <div class="document-summary" style="flex:1">        (block stack)
//	      <div class="the_subtotal">Subtotal …</div>
//	      <div class="the_totals">Volume Discount …</div>
//	      … Shipping, Tax, grand Total …
//
// When this block overflowed the bottom of the last line-item page, folio kept
// only "Subtotal" and dropped Volume Discount / Shipping / Tax / the grand
// Total — the PO's total amount silently disappeared from the PDF.
func TestFlexItemOverflowNotDropped(t *testing.T) {
	rows := []string{"SUMSubtotal", "SUMDiscount", "SUMShipping", "SUMTax", "SUMGrandTotal"}

	build := func(noBreak bool) string {
		var b strings.Builder
		b.WriteString(`<html><head><style>
			body{margin:0;padding:0}
			.nb{page-break-inside:avoid;break-inside:avoid}
			.spacer{height:600pt}
			.main{display:flex}
			.note{flex:2}
			.summary{flex:1}
			.row{display:flex;margin-top:5pt}
			.c2{border:1px solid #ccc;padding:5px;margin-left:4px;flex:1}
			.c3{border:1px solid #ccc;padding:5px;margin-left:4px;text-align:right;width:90px}
		</style></head><body>`)
		b.WriteString(`<div class="spacer">top</div>`)
		cls := "wrap"
		if noBreak {
			cls = "wrap nb"
		}
		fmt.Fprintf(&b, `<div class="%s"><div class="main"><div class="note"></div><div class="summary">`, cls)
		for _, r := range rows {
			fmt.Fprintf(&b, `<div class="row"><span class="c2">%s</span><span class="c3">$1</span></div>`, r)
		}
		b.WriteString(`</div></div></div></body></html>`)
		return b.String()
	}

	t.Run("no rows dropped when summary overflows the page", func(t *testing.T) {
		pdf, err := renderHTMLToPDF(build(false))
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		for _, r := range rows {
			if findTextY(pdf, r) < 0 {
				t.Errorf("%s missing — flex item overflow was dropped", r)
			}
		}
	})

	t.Run("page-break-inside:avoid keeps the block together on one page", func(t *testing.T) {
		pdf, err := renderHTMLToPDF(build(true))
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		// All rows present …
		first := findTextY(pdf, rows[0])
		last := findTextY(pdf, rows[len(rows)-1])
		if first < 0 || last < 0 {
			t.Fatalf("rows missing: first=%.1f last=%.1f", first, last)
		}
		// … and the whole no-break block relocated to a fresh page, so the
		// rows are stacked close together (a few line-heights apart), not
		// split across a page boundary (which would put them ~hundreds of pt
		// apart in opposite page coordinate ranges).
		if first-last > 200 {
			t.Errorf("no-break block appears split (first=%.1f last=%.1f) — page-break-inside:avoid not honored", first, last)
		}
	})
}
