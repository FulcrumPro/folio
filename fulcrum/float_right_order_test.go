// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestFloatRightRunReversesDOMOrder pins the v0.9.1-fulcrum.24 patch: a run of
// consecutive `float:right` siblings stacks from the right edge, so the FIRST
// element in DOM order ends up RIGHTMOST. folio laid a float run out in DOM
// order left-to-right, so the .NET DocGen Job traveler header — which floats its
// "Quantity" then "Production Due" columns right — rendered them swapped versus
// Chrome (Quantity must sit to the RIGHT of Production Due).
func TestFloatRightRunReversesDOMOrder(t *testing.T) {
	// Two float:right columns in DOM order [QTY, DUE]. Chrome renders DUE then
	// QTY left-to-right (QTY rightmost). Assert QTY's x is greater than DUE's.
	src := `<html><head><style>body{margin:0;padding:0}
		.wrap{width:400pt}
		.col{float:right;width:120pt}
	</style></head><body>
		<div class="wrap">
			<div class="col">QTY</div>
			<div class="col">DUE</div>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	qty := findTextX(pdf, "QTY")
	due := findTextX(pdf, "DUE")
	if qty < 0 || due < 0 {
		t.Fatalf("markers missing: QTY=%.1f DUE=%.1f", qty, due)
	}
	t.Logf("QTY x=%.1f  DUE x=%.1f (QTY must be rightmost)", qty, due)
	if qty <= due {
		t.Errorf("QTY x=%.1f not to the right of DUE x=%.1f — float:right run not reversed (columns appear swapped vs Chrome)", qty, due)
	}
}
