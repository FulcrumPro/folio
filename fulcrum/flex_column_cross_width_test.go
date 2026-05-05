package fulcrum

import (
	"testing"
)

// TestFlexColumnChildExplicitWidth pins the v0.7.1-fulcrum.11 patch:
// CSS `width: 250px` on a flex child inside a `flex-direction: column`
// container is the CROSS-axis size (per CSS Flexbox spec — `width` /
// `height` always refer to physical dimensions, while flex-basis
// tracks the flex container's main axis).
//
// folio v0.7.1's converter_flex.go consumed CSS `width` as flex-basis
// regardless of direction. In a column flex that meant `width: 250px`
// became a vertical basis (which is meaningless on the main axis when
// the item also has flex-grow — basis is just absorbed) AND the Div's
// own width unit was cleared, so the box stretched to the full cross
// axis. The .NET DocGen v3 BILLING box uses
//
//	.v3-info-wrapper  { display: flex }
//	.v3-info-contain1 { flex: 1; display: flex; flex-direction: column }
//	.v3-pdf-details-left { flex-grow: 1; width: 250px; border:… }
//
// Pre-patch the bordered box stretched edge-to-edge on a US-Letter
// page (~612pt content width). Post-patch it sits at 250pt as
// Chromium does.
func TestFlexColumnChildExplicitWidth(t *testing.T) {
	src := `<html><head><style>
		body { margin: 0; padding: 0; }
		.outer { width: 600pt; }
		.col { display: flex; flex-direction: column; }
		.boxed { flex-grow: 1; width: 250pt; background: #ff0; height: 50pt; }
	</style></head><body>
		<div class="outer"><div class="col">
			<div class="boxed">B</div>
		</div></div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	widths := extractFilledRectWidths(pdf)
	if len(widths) < 1 {
		t.Fatalf("expected at least 1 filled rect, got %d", len(widths))
	}
	w := widths[0]
	// Allow ~1pt slack for rounding. Pre-patch the rect was ~600pt
	// (full column width); post-patch it's the explicit 250pt.
	if w < 248 || w > 252 {
		t.Errorf(".boxed rect width %.1fpt; expected ~250pt for `width: 250pt` on a column-flex child. Outside that range usually means the converter consumed CSS `width` as flex-basis even in column direction, then cleared the Div's own width and let the cross-axis stretch take over.",
			w)
	}
}
