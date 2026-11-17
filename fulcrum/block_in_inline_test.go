// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// TestBlockInsideInlineBreaksLines pins the v0.9.1-fulcrum.36 patch: a
// block-level box inside an inline element splits the inline (CSS 2.1
// §9.2.1.1), so the text before the block, the block, and the text after it
// each start a line of their own.
//
// walkChildren buffered an inline element whole and collected its runs, which
// ignore a descendant's display. The .NET DocGen certification bill-to address
// is `<span class=""><span class="d-block">Name</span><span
// class="d-block">Street</span>…</span>` with `.d-block { display: block }`;
// it printed on one line after the "Bill to Address" label instead of one line
// each.
func TestBlockInsideInlineBreaksLines(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
	}{
		{
			name: "display:block spans inside a span",
			html: `<style>.d-block { display: block }</style>
<div>
  <span><b>Bill to Address</b></span>
  <span class="">
    <span class="d-block">Acme Machining Works Company</span>
    <span class="d-block">12345 NW Elm St</span>
    <span class="d-block">Sometown, OR 97000</span>
  </span>
</div>`,
			want: []string{"Bill to Address", "Acme Machining Works Company", "12345 NW Elm St", "Sometown, OR 97000"},
		},
		{
			name: "text around a div inside nested spans",
			html: `<div>before <span><em>in <span><div>block</div></span> after</em></span> end</div>`,
			want: []string{"before in", "block", "after end"},
		},
		{
			name: "inline-block does not split",
			html: `<div>a <span>b <span style="display:inline-block">c</span> d</span> e</div>`,
			want: []string{"a b c d e"},
		},
		{
			name: "a link keeps its content inline",
			html: `<div>a <a href="https://example.com">b <span style="display:block">c</span></a> d</div>`,
			want: []string{"a b c d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf, err := renderHTMLToPDF(`<html><body>` + tt.html + `</body></html>`)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			var want []string
			for _, l := range tt.want {
				want = append(want, strings.ReplaceAll(l, " ", ""))
			}
			if got := pageLines(t, pdf); !slices.Equal(got, want) {
				t.Errorf("lines = %q, want %q", got, want)
			}
		})
	}
}

// pageLines reads the first page as lines of text, top to bottom, joining the
// spans on one baseline left to right. Spaces are left out: the reader
// estimates standard-font span widths, too roughly to place word gaps.
func pageLines(t *testing.T, pdf []byte) []string {
	t.Helper()
	spans := pageSpans(t, pdf)
	slices.SortStableFunc(spans, func(a, b reader.TextSpan) int {
		if math.Abs(a.Y-b.Y) > 1 {
			return cmp.Compare(b.Y, a.Y)
		}
		return cmp.Compare(a.X, b.X)
	})
	var lines []string
	for i, s := range spans {
		if i == 0 || math.Abs(s.Y-spans[i-1].Y) > 1 {
			lines = append(lines, "")
		}
		lines[len(lines)-1] += strings.ReplaceAll(s.Text, " ", "")
	}
	return lines
}

// TestInlineBlockHoldingBlockStaysOneBox pins that only a display:inline box
// splits around a block. An inline-block is an atomic inline box: the blocks
// it holds lay out inside it, so the text on either side of it stays on one
// line, and so does the inline element that holds it.
func TestInlineBlockHoldingBlockStaysOneBox(t *testing.T) {
	tests := []struct{ name, html string }{
		{"a div inside an inline-block", `<div>a <span style="display:inline-block;width:100px">b<div>c</div></span> d</div>`},
		{"display:block spans inside an inline-block", `<div>a <span style="display:inline-block;width:100px"><span style="display:block">b</span><span style="display:block">c</span></span> d</div>`},
		{"an inline-block holding a div inside a span", `<div>a <span>x <span style="display:inline-block;width:100px"><div>c</div></span> y</span> d</div>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf, err := renderHTMLToPDF(`<html><body>` + tt.html + `</body></html>`)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			lines := pageLines(t, pdf)
			if !slices.ContainsFunc(lines, func(l string) bool {
				return strings.HasPrefix(l, "a") && strings.HasSuffix(l, "d")
			}) {
				t.Errorf("lines = %q, want a line that starts with %q and ends with %q", lines, "a", "d")
			}
		})
	}
}
