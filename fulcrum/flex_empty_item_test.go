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

// TestEmptyFlexChildKeepsItemCount pins the v0.9.1-fulcrum.13 patch: an empty
// (but visible, not display:none) element child of a flex container is still a
// flex item per CSS Flexbox §4 — it generates a zero-size item box that
// participates in justify-content distribution.
//
// The v3 commerce footer is `display:flex; justify-content:space-between` with
// an empty <div class="last-modified"></div> followed by the "Powered By //
// fulcrum" block. Chrome keeps the empty first item, so space-between holds the
// Powered-By block at the right edge. folio's converter dropped element
// children that rendered no content (len(childElems)==0 → continue), leaving a
// single item — and space-between with one item packs at flex-start, so the
// footer rendered hard left instead of right.
//
// The converter now synthesizes a zero-size placeholder for an empty visible
// element child. display:none and non-visual head elements (script/style/etc)
// still generate no item.
func TestEmptyFlexChildKeepsItemCount(t *testing.T) {
	const css = `body{margin:0;padding:0;width:600pt}
		.footer{display:flex;justify-content:space-between}`

	t.Run("empty first item holds sibling at right edge", func(t *testing.T) {
		src := `<html><head><style>` + css + `</style></head><body>
			<div class="footer"><div class="last-modified"></div><div>RIGHT</div></div>
		</body></html>`
		pdf, err := renderHTMLToPDF(src)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		x := findTextX(pdf, "RIGHT")
		// Page is US Letter (612pt) with a ~72pt default margin; the right
		// edge sits well past mid-page. flex-start collapse lands it at ~72.
		if x < 300 {
			t.Errorf("RIGHT at x=%.1f — empty first flex item was dropped, collapsing space-between to flex-start", x)
		}
	})

	t.Run("display:none child does NOT become an item", func(t *testing.T) {
		// With the first child display:none there is genuinely one flex item;
		// space-between then correctly packs it at flex-start (left).
		src := `<html><head><style>` + css + `
			.hidden{display:none}
			</style></head><body>
			<div class="footer"><div class="hidden"></div><div>SOLO</div></div>
		</body></html>`
		pdf, err := renderHTMLToPDF(src)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		x := findTextX(pdf, "SOLO")
		if x > 300 {
			t.Errorf("SOLO at x=%.1f — display:none child was wrongly synthesized into a flex item", x)
		}
	})
}

// findTextX returns the X coordinate of the Td preceding the first `(text) Tj`
// show operator across all FlateDecode'd content streams. Returns -1 if not
// found. Companion to findTextY in helpers_test.go.
func findTextX(pdf []byte, text string) float64 {
	tjRe := regexp.MustCompile(`([\d.]+) ([\d.]+) Td\s*\(([^)]+)\)\s*Tj`)
	rest := pdf
	for {
		i := bytes.Index(rest, []byte("\nstream\n"))
		if i < 0 {
			return -1
		}
		j := bytes.Index(rest[i:], []byte("\nendstream"))
		if j < 0 {
			return -1
		}
		raw := rest[i+8 : i+j]
		if zr, err := zlib.NewReader(bytes.NewReader(raw)); err == nil {
			decoded, _ := io.ReadAll(zr)
			zr.Close()
			for _, m := range tjRe.FindAllStringSubmatch(string(decoded), -1) {
				if m[3] == text {
					if x, e := strconv.ParseFloat(m[1], 64); e == nil {
						return x
					}
				}
			}
		}
		rest = rest[i+j+10:]
	}
}
