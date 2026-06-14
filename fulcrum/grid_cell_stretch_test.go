// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strconv"
	"testing"
)

// TestGridCellStretchFillsCell pins the v0.9.1-fulcrum.22 patch: a grid cell's
// element must stretch to the row height under align-items: stretch (the CSS
// grid default), so its background/border covers the whole cell — not just its
// content.
//
// folio's grid only *positioned* cell content within the cell (offset 0 for
// stretch) and never resized it. The .NET DocGen v3 line-item index badge
// (`.order-background { background:#273e6c }`, a short cell in a tall
// multi-line row) drew its dark background only behind the number, leaving the
// rest of the cell white. Chrome fills the whole cell.
func TestGridCellStretchFillsCell(t *testing.T) {
	src := `<html><head><style>
		body{margin:0;padding:0}
		.row{display:grid;grid-template-columns:40pt 200pt}
		.num{background:#273e6c;display:flex;justify-content:center;align-items:center;color:#fff}
		.body{padding:5pt}
	</style></head><body>
		<div class="row"><div class="num">7</div>
			<div class="body">Line one<br>Line two<br>Line three<br>Line four</div></div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// The ~40pt-wide number cell's background fill rectangle should span the
	// whole row (4 text lines ≈ 50pt+), not just the single number line (~14pt).
	bgH := narrowRectHeight(pdf, 60)
	t.Logf("number-cell background rect height = %.1f", bgH)
	if bgH < 40 {
		t.Errorf("badge background height %.1f — cell did not stretch to the row (background only behind the number)", bgH)
	}
}

// TestGridCellStretchCentersContent pins the v0.9.1-fulcrum.23 patch: when a
// grid cell is a flex with border-radius (so folio wraps it in a Div), the flex
// must fill the stretched wrapper so its align-items:center centers content
// vertically. Without the wrapper passing its forced height to the flex, the
// number stayed at the top of the (filled) cell.
func TestGridCellStretchCentersContent(t *testing.T) {
	// border-radius forces the convertFlex wrapper-Div path (the bug's domain).
	src := `<html><head><style>
		body{margin:0;padding:0}
		.row{display:grid;grid-template-columns:40pt 200pt}
		.num{background:#273e6c;border-radius:2px;display:flex;justify-content:center;align-items:center;color:#fff}
		.body{padding:5pt}
	</style></head><body>
		<div class="row"><div class="num">7</div>
			<div class="body">L1<br>L2<br>L3<br>L4<br>L5<br>L6</div></div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// The badge '7' baseline should sit near the middle of the row, not at the
	// top. Row content runs from L1 (top) to L6 (bottom); '7' y should be within
	// the middle band, not within a couple line-heights of the top.
	seven := findTextY(pdf, "7")
	l1 := findTextY(pdf, "L1")
	l6 := findTextY(pdf, "L6")
	if seven < 0 || l1 < 0 || l6 < 0 {
		t.Fatalf("markers missing: 7=%.1f L1=%.1f L6=%.1f", seven, l1, l6)
	}
	mid := (l1 + l6) / 2
	t.Logf("'7' y=%.1f  rowMid=%.1f  L1(top)=%.1f L6(bottom)=%.1f", seven, mid, l1, l6)
	// PDF y is bottom-origin: top line L1 has the LARGEST y. '7' top-aligned
	// would be near l1; centered is near mid. Require it past the upper third.
	if seven > l1-(l1-l6)/3 {
		t.Errorf("'7' y=%.1f is near the top (L1=%.1f) — flex content not vertically centered in the stretched cell", seven, l1)
	}
}

// narrowRectHeight returns the height of the widest `re` rectangle whose width
// is below maxW (the narrow index-badge column), across decompressed streams.
func narrowRectHeight(pdf []byte, maxW float64) float64 {
	re := regexp.MustCompile(`([\d.]+) ([\d.-]+) ([\d.]+) (-?[\d.]+) re`)
	best := 0.0
	rest := pdf
	for {
		i := bytes.Index(rest, []byte("\nstream\n"))
		if i < 0 {
			return best
		}
		j := bytes.Index(rest[i:], []byte("\nendstream"))
		if j < 0 {
			return best
		}
		if zr, err := zlib.NewReader(bytes.NewReader(rest[i+8 : i+j])); err == nil {
			dec, _ := io.ReadAll(zr)
			zr.Close()
			for _, m := range re.FindAllStringSubmatch(string(dec), -1) {
				w, _ := strconv.ParseFloat(m[3], 64)
				h, _ := strconv.ParseFloat(m[4], 64)
				if h < 0 {
					h = -h
				}
				if w < maxW && h > best {
					best = h
				}
			}
		}
		rest = rest[i+j+10:]
	}
}
