package fulcrum

import (
	"testing"
)

// TestColumnFlexGrowOnlyFiresWithDefiniteHeight pins the
// v0.7.1-fulcrum.7 patch: a flex-direction:column container without
// an explicit height (and without a definite cross-size from a parent
// row) must NOT distribute "remaining space" to flex-grow children.
//
// CSS Flexbox §9.7 — flex-grow on the main axis only applies when the
// container has definite main-axis size. For column-direction the
// main axis is vertical, so "definite main size" = explicit height.
//
// Symptom in production: the .NET v3 sales/purchasing templates have
//
//   .v3-info-wrapper { display: flex }            (row, no height)
//   .v3-info-contain1, .v3-info-contain2 {
//       flex: 1; display: flex; flex-direction: column;
//   }
//   .v3-pdf-details-left, .v3-pdf-details-right {
//       flex-grow: 1; border: 1px solid;
//   }
//
// Before the patch, the column flex received the available page
// height from the row and used it to grow the bordered children to
// nearly the bottom of the page — making the BILLING and NOTE boxes
// stretch ~70% of the page tall instead of sitting at content size.
// jsreport (Chromium) renders these content-sized.
//
// We observe the fix by rendering the same nested-flex shape twice
// — once with a tall first column and a one-line second column —
// and asserting the rendered PDF is shorter than it would be if the
// boxes had stretched to fill the page.
func TestColumnFlexGrowOnlyFiresWithDefiniteHeight(t *testing.T) {
	src := `<html><head><style>
		body { margin: 0; padding: 50pt; }
		.row { display: flex; }
		.col { flex: 1; display: flex; flex-direction: column; }
		.box { flex-grow: 1; border: 1px solid #888; padding: 8px; }
	</style></head><body>
		<div class="row">
			<div class="col">
				<div>label1</div>
				<div class="box">
					<div>line a</div>
					<div>line b</div>
					<div>line c</div>
				</div>
			</div>
			<div class="col">
				<div>label2</div>
				<div class="box">
					<div>just one line</div>
				</div>
			</div>
		</div>
		<p>after the row</p>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// PDF Y is bottom-up: label1 has a higher Y than "after the row"
	// because "after" is below it on the page. With content-sized
	// boxes (the patched behavior), the vertical span between them is
	// ~70-100pt: one label-line, three box content-lines, plus box
	// padding+border. Without the patch, the boxes either overflow
	// to a second page or push "after" much further below — either
	// way the rendered position of "after" on page 1 is hugely
	// different from the patched case.
	yLabel := findTextY(pdf, "label1")
	yAfter := findTextY(pdf, "after")
	if yLabel < 0 || yAfter < 0 {
		t.Fatalf("could not locate text positions: label1=%v, after=%v", yLabel, yAfter)
	}
	gap := yLabel - yAfter
	if gap < 60 || gap > 150 {
		t.Errorf("vertical span between label1 (y=%.1f) and 'after the row' (y=%.1f) is %.1fpt; expected 60-150pt for content-sized boxes. Outside that range usually means the column flex's flex-grow fired despite indefinite main-axis size.",
			yLabel, yAfter, gap)
	}
}

// relativeYPositionGap searches a PDF's content streams for two text
// strings and returns the absolute difference in their Y-coordinates
// (`y` from a `TJ` / `Tj` text-show operation's preceding `Td`).
// Returns -1 when either string isn't found. PDF Y-axis is bottom-up,
// so "earlier in document" means HIGHER Y. We return abs(diff) so
// callers don't have to know that.
func relativeYPositionGap(pdf []byte, markerA, markerB string) float64 {
	yA := findTextY(pdf, markerA)
	yB := findTextY(pdf, markerB)
	if yA < 0 || yB < 0 {
		return -1
	}
	if yA > yB {
		return yA - yB
	}
	return yB - yA
}
