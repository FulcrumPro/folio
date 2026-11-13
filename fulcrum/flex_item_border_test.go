// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// TestFlexItemSpanPaintsBorder pins the v0.9.1-fulcrum.36 patch: an inline
// element that is a flex item paints its border (CSS Flexbox §4 blockifies
// every flex item), across the full width the item grew to.
//
// blockifyInlineBoxChild routed a flex-item span through the block path only
// when it had a border-radius, so a plain border was dropped with the rest of
// the box; and the blockified box shrank to its text, so a flex-grow:1 item's
// border would have spanned only its text. The .NET DocGen certification
// draws its Signature and Date lines this way: a border-bottom on a
// flex-grow:1 span inside a display:flex span, itself a flex item.
func TestFlexItemSpanPaintsBorder(t *testing.T) {
	const page = `<html><body><div style="display: flex; justify-content: space-between; width: 400pt">
  <span style="display: flex; flex-grow: 2; align-items: flex-end;">
    <span style="padding-right: 8px;">Signature:</span>
    <span style="flex-grow: 1; margin-right: .5in; border-bottom: 1px solid black;">Jo</span>
  </span>
  <span style="display: flex; flex-grow: 1;">
    <span style="padding-right: 8px;">Date:</span>
    <span style="flex-grow: 1; border-bottom: 1px solid black;">1/2/2026</span>
  </span>
</div></body></html>`
	pdf, err := renderHTMLToPDF(page)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p, err := r.Page(0)
	if err != nil {
		t.Fatalf("page 0: %v", err)
	}
	ops, err := p.PathOps()
	if err != nil {
		t.Fatalf("path ops: %v", err)
	}
	var rules []float64
	for i := 1; i < len(ops); i++ {
		if ops[i].Type != reader.PathLine || ops[i-1].Type != reader.PathMove {
			continue
		}
		a, b := ops[i-1].Points[0], ops[i].Points[0]
		if math.Abs(a[1]-b[1]) < 0.01 {
			rules = append(rules, math.Abs(b[0]-a[0]))
		}
	}
	if len(rules) != 2 {
		t.Fatalf("painted %d horizontal rules %v, want 2", len(rules), rules)
	}
	// In the 400pt row each growing item gets well over 80pt, far more than
	// the "Jo" or "1/2/2026" above its rule.
	for _, w := range rules {
		if w < 80 {
			t.Errorf("rule is %.1fpt wide; a growing item's border spans its grown width, not its text", w)
		}
	}
}
