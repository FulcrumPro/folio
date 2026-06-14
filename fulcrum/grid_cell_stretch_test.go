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
