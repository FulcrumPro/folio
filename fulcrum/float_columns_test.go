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

// TestFloatColumnsLayOutSideBySide pins the v0.9.1-fulcrum.18 patch: a run of
// consecutive sibling floats (a float-based column layout) must lay out
// side-by-side, not stacked at the container's left edge.
//
// folio's float positioning never offsets one float past another, so a run of
// same-side floats all rendered at x=0 — drawn on top of each other,
// illegible. The .NET DocGen Certification (CoC) header uses
// `.three-columns { float: left; width: 33.3% }` for its Customer / Comments /
// Date columns; folio rendered them as one stacked, unreadable blob.
//
// The converter now detects a run of 2+ consecutive sibling floats in a block
// and lays them out as a flex row (each float's CSS width → the flex item's
// basis). A single float is left as a real float (text wraps around it).
func TestFloatColumnsLayOutSideBySide(t *testing.T) {
	src := `<html><head><style>
		body{margin:0;padding:0}
		.wrap{padding:0 8px}
		.col{width:33.3%;float:left;padding-right:10px}
	</style></head><body>
		<div class="wrap">
			<div class="col">COLA aaa</div>
			<div class="col">COLB bbb</div>
			<div class="col">COLC ccc</div>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Every `X Y Td` on the columns' baseline. Stacked-overlap puts all three
	// columns at the same (left) x; side-by-side spreads them across the row.
	xs := tdXsByRow(pdf)
	var best []float64
	for _, row := range xs {
		if len(row) > len(best) {
			best = row
		}
	}
	if len(best) < 3 {
		t.Fatalf("expected >=3 text positions on the column row, got %v", best)
	}
	lo, hi := best[0], best[0]
	for _, x := range best {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	t.Logf("column-row Td xs spread: %.1f .. %.1f", lo, hi)
	// Three 1/3 columns across a ~600pt page span >300pt; stacked columns all
	// sit within a few pt of the left edge.
	if hi-lo < 150 {
		t.Errorf("float columns stacked: all text within %.1fpt (xs=%v) — expected 3 columns spread across the row", hi-lo, best)
	}
}

// tdXsByRow groups the X of every `X Y Td` by its Y (rounded), across all
// FlateDecode'd content streams — used to find the columns sharing a baseline.
func tdXsByRow(pdf []byte) map[int][]float64 {
	tdRe := regexp.MustCompile(`([\d.]+) ([\d.]+) Td`)
	rows := map[int][]float64{}
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
		if zr, err := zlib.NewReader(bytes.NewReader(rest[i+8 : i+j])); err == nil {
			dec, _ := io.ReadAll(zr)
			zr.Close()
			for _, m := range tdRe.FindAllStringSubmatch(string(dec), -1) {
				x, _ := strconv.ParseFloat(m[1], 64)
				y, _ := strconv.ParseFloat(m[2], 64)
				k := int(y + 0.5)
				rows[k] = append(rows[k], x)
			}
		}
		rest = rest[i+j+10:]
	}
	return rows
}
