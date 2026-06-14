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

// TestBackgroundPositionKeywordOrder pins the v0.9.1-fulcrum.22 patch: the
// two-value background-position keyword syntax is order-independent —
// "top right" means horizontal:right, vertical:top, same as "right top".
//
// folio assigned the two keywords positionally (first→x, second→y), so
// "top right" became x=top(0%), y=right — pinning the image to the LEFT edge.
// The .NET DocGen v3 header logo uses `background-position: top right` with
// `background-size: contain` to right-align the logo within its box; folio put
// it at the left, ~50pt short of the contact column it should align with.
func TestBackgroundPositionKeywordOrder(t *testing.T) {
	// A 400pt box, contained background image positioned "top right": the image
	// must be drawn against the box's right edge, not the left.
	src := `<html><head><style>
		body{margin:0;padding:0}
		.box{width:400pt;height:80pt;background-image:url(https://placehold.co/240x100/png?text=L);
		     background-repeat:no-repeat;background-size:contain;background-position:top right}
	</style></head><body><div class="box"></div></body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	x, w, ok := largestImageXW(pdf)
	if !ok {
		t.Skip("no image drawn (asset fetch unavailable in this environment)")
	}
	right := x + w
	t.Logf("logo image x=%.1f w=%.1f right=%.1f (box left margin ≈72, right ≈472)", x, w, right)
	// box spans ~72..472 (400pt + default margin). top-right ⇒ image right edge
	// near 472; the bug pinned it left (right edge ≈ 72+w).
	if right < 300 {
		t.Errorf("image right edge=%.1f — background-position:top right placed it at the left", right)
	}
}

// largestImageXW finds the `w 0 0 h x y cm` matrix of the largest drawn image.
func largestImageXW(pdf []byte) (x, w float64, ok bool) {
	re := regexp.MustCompile(`([\d.]+) 0 0 ([\d.]+) ([\d.]+) ([\d.]+) cm`)
	best := 0.0
	rest := pdf
	for {
		i := bytes.Index(rest, []byte("\nstream\n"))
		if i < 0 {
			return x, w, ok
		}
		j := bytes.Index(rest[i:], []byte("\nendstream"))
		if j < 0 {
			return x, w, ok
		}
		if zr, err := zlib.NewReader(bytes.NewReader(rest[i+8 : i+j])); err == nil {
			dec, _ := io.ReadAll(zr)
			zr.Close()
			for _, m := range re.FindAllStringSubmatch(string(dec), -1) {
				cw, _ := strconv.ParseFloat(m[1], 64)
				ch, _ := strconv.ParseFloat(m[2], 64)
				cx, _ := strconv.ParseFloat(m[3], 64)
				if cw > 50 && ch > 20 && cw > best {
					best, x, w, ok = cw, cx, cw, true
				}
			}
		}
		rest = rest[i+j+10:]
	}
}
