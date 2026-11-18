// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"slices"
	"testing"
)

// TestCommentInsideInlineKeepsOneLine pins the v0.9.1-fulcrum.38 patch: an
// HTML comment does not end an inline run. walkChildren treated a comment
// node as a block-level child, so "Net <!-- c --> 30 terms" flushed "Net" as
// a paragraph of its own and printed "30 terms" on the next line. A browser
// prints one line.
func TestCommentInsideInlineKeepsOneLine(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
	}{
		{"between text in a div", `<div>Net <!-- c --> 30 terms</div>`, []string{"Net30terms"}},
		{"between text in a cell", `<table><tr><td>Net <!-- c --> 30 terms</td></tr></table>`, []string{"Net30terms"}},
		{"between spans with whitespace", `<div><span>Net</span> <!-- c --> <span>30 terms</span></div>`, []string{"Net30terms"}},
		{"between blocks", `<div>a</div><!-- c --><div>b</div>`, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf, err := renderHTMLToPDF(`<html><body>` + tt.html + `</body></html>`)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := pageLines(t, pdf); !slices.Equal(got, tt.want) {
				t.Errorf("lines = %q, want %q", got, tt.want)
			}
		})
	}
}
