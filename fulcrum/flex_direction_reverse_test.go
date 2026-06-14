// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import "testing"

// TestFlexRowReverse pins the v0.9.1-fulcrum.20 patch: `flex-direction:
// row-reverse` must lay items out from the main-end (right edge), and
// `justify-content: right` / `left` must map to flex-end / flex-start.
//
// folio mapped row-reverse to a plain row and didn't recognize the
// positional `right` / `left` keywords (they fell through to flex-start). The
// .NET DocGen v3 commerce header right-aligns its logo with
// `.logo.container { display:flex; flex-direction:row-reverse }`; folio left it
// stuck against the title at the left of the header instead of top-right.
//
// row-reverse is implemented as: reverse the child order + flip
// justify-content start/end (the reversed-axis equivalence).
func TestFlexRowReverse(t *testing.T) {
	t.Run("row-reverse pushes a single item to the right", func(t *testing.T) {
		src := `<html><head><style>body{margin:0;padding:0}
			.c{display:flex;flex-direction:row-reverse;width:400pt}
		</style></head><body><div class="c"><div>LOGO</div></div></body></html>`
		pdf, err := renderHTMLToPDF(src)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		x := findTextX(pdf, "LOGO")
		if x < 250 {
			t.Errorf("LOGO at x=%.1f — row-reverse not honored (should be near the right edge of the 400pt row)", x)
		}
	})

	t.Run("row-reverse reverses item order along the main axis", func(t *testing.T) {
		// In row-reverse, the FIRST DOM child ends up rightmost. AAA before BBB
		// in source ⇒ AAA to the right of BBB.
		src := `<html><head><style>body{margin:0;padding:0}
			.c{display:flex;flex-direction:row-reverse;width:400pt}
		</style></head><body><div class="c"><div>AAA</div><div>BBB</div></div></body></html>`
		pdf, err := renderHTMLToPDF(src)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		ax := findTextX(pdf, "AAA")
		bx := findTextX(pdf, "BBB")
		if ax < 0 || bx < 0 {
			t.Fatalf("markers missing: AAA=%.1f BBB=%.1f", ax, bx)
		}
		if ax <= bx {
			t.Errorf("AAA.x=%.1f BBB.x=%.1f — first DOM child should be rightmost under row-reverse", ax, bx)
		}
	})

	t.Run("justify-content:right maps to flex-end", func(t *testing.T) {
		render := func(j string) float64 {
			src := `<html><head><style>body{margin:0;padding:0}
				.c{display:flex;justify-content:` + j + `;width:400pt}
			</style></head><body><div class="c"><div>ONLY</div></div></body></html>`
			pdf, err := renderHTMLToPDF(src)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			// Rightmost text Td across the doc = the (single) item's x.
			// Robust to Tj vs TJ-array shows, which findTextX is not.
			maxX := -1.0
			for _, row := range tdXsByRow(pdf) {
				for _, x := range row {
					if x > maxX {
						maxX = x
					}
				}
			}
			return maxX
		}
		// right should land the item far past where flex-start (left) does.
		left := render("left")
		right := render("right")
		t.Logf("ONLY x: justify left=%.1f right=%.1f", left, right)
		if right-left < 150 {
			t.Errorf("justify-content:right (x=%.1f) not right-aligned vs left (x=%.1f) — `right` not mapped to flex-end", right, left)
		}
	})
}
