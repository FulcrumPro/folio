package fulcrum

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strconv"
	"testing"
)

// TestFlexItemMinMaxWidthPercent pins the v0.7.1-fulcrum.10 patch:
// CSS `min-width: 50%` and `max-width: 55%` on a flex item must be
// honored during the flex grow/shrink distribution. CSS Flexbox §9.7
// step 4: after the basis-and-grow pass, items are clamped to their
// min/max main size and the resulting violation is redistributed
// among the remaining unclamped items.
//
// folio v0.7.1's resolveGrowShrink stopped after the basis-and-grow
// pass and never clamped. The .NET DocGen v3 templates' inner
// BILLING/SHIPPING flex row defines `.contain-left { flex: 1;
// min-width: 50%; max-width: 55% }` next to `.contain-right { flex:
// 2 }` — without the clamp, the LEFT column was given its 1/3
// flex-grow share (~33%) instead of the 50% floor min-width called
// for, making addresses like "Acme Industries" wrap onto two lines
// where Chromium kept them on one.
//
// Patch: track resolved min/max main-size on FlexItem, apply a
// single-pass CSS-spec clamp at the end of resolveGrowShrink that
// freezes clamped items and redistributes the delta to unclamped
// growers. Single pass covers every shape our v3 templates use; CSS
// spec calls for iteration but no template here surfaces a case
// that needs it.
func TestFlexItemMinMaxWidthPercent(t *testing.T) {
	src := `<html><head><style>
		body { margin: 0; padding: 0; }
		.outer { width: 300pt; }
		.row { display: flex; }
		.left  { flex: 1; min-width: 50%; max-width: 55%; background: #ff0; height: 50pt; }
		.right { flex: 2; background: #0f0; height: 50pt; }
	</style></head><body>
		<div class="outer"><div class="row">
			<div class="left">L</div>
			<div class="right">R</div>
		</div></div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	widths := extractFilledRectWidths(pdf)
	// We expect two filled rects (the yellow .left and green .right
	// backgrounds). The .left should be at least 50% of the outer
	// (150pt) per its min-width. Pre-patch, it was distributed ~1/3
	// (~100pt) per the flex-grow ratio, ignoring min-width.
	if len(widths) < 2 {
		t.Fatalf("expected at least 2 filled rects, got %d", len(widths))
	}
	leftW := widths[0]
	if leftW < 145 || leftW > 170 {
		t.Errorf(".left rect width %.1fpt; expected 145-170pt for min-width:50%%, max-width:55%% on a 300pt outer. Outside that range usually means the flex grow/shrink ignored the CSS min/max.",
			leftW)
	}
}

// extractFilledRectWidths walks the PDF's content streams and returns
// the widths of every filled rectangle (`X Y W H re f`) in document
// order. Background-color rectangles drawn by folio for elements with
// `background:` are filled rects; the helper lets the test inspect
// item widths without doing any layout-engine surgery.
func extractFilledRectWidths(pdf []byte) []float64 {
	rectRe := regexp.MustCompile(`(\d+(?:\.\d+)?) (\d+(?:\.\d+)?) (\d+(?:\.\d+)?) (\d+(?:\.\d+)?) re\s*f`)
	var widths []float64
	rest := pdf
	for {
		i := bytes.Index(rest, []byte("\nstream\n"))
		if i < 0 {
			break
		}
		j := bytes.Index(rest[i:], []byte("\nendstream"))
		if j < 0 {
			break
		}
		raw := rest[i+8 : i+j]
		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err == nil {
			decoded, _ := io.ReadAll(zr)
			zr.Close()
			for _, m := range rectRe.FindAllStringSubmatch(string(decoded), -1) {
				if w, err := strconv.ParseFloat(m[3], 64); err == nil {
					widths = append(widths, w)
				}
			}
		}
		rest = rest[i+j+10:]
	}
	return widths
}
