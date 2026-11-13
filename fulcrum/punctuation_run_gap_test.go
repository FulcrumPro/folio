// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// TestPunctuationRunKeepsFollowingSpace pins the v0.9.1-fulcrum.36 patch:
// text split into runs at a punctuation boundary lays out like the same text
// in one run.
//
// measureWords appends a run's leading punctuation to the previous run's last
// word ("Rd" + ", Suite" → "Rd,"), and that word kept the zero gap that glued
// "Rd" to the comma. The space after the comma was lost: the .NET DocGen
// company strip, `<span> 12345 NW Cedar Hollow Rd</span><span>, Suite C</span>`,
// printed "Rd,Suite C". Punctuation after a space ("foo " + ",bar") was also
// pulled back onto the previous word.
func TestPunctuationRunKeepsFollowingSpace(t *testing.T) {
	tests := []struct {
		name        string
		runs, whole string
		word        string // the word after the boundary
	}{
		{"space after a merged comma", `<span>Cedar Hollow Rd</span><span>, Suite C</span>`, `Cedar Hollow Rd, Suite C`, "Suite"},
		{"run of only punctuation, then a spaced run", `<span>end</span><span>)</span><span> next</span>`, `end) next`, "next"},
		{"space before punctuation", `<span>foo </span><span>,bar</span>`, `foo ,bar`, ",bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := func(body string) float64 {
				pdf, err := renderHTMLToPDF(`<html><body><p>` + body + `</p></body></html>`)
				if err != nil {
					t.Fatalf("render: %v", err)
				}
				// Kerning can split a word into several shows; its first
				// show starts where the word does.
				for _, s := range pageSpans(t, pdf) {
					if s.Text != "" && strings.HasPrefix(tt.word, s.Text) {
						return s.X
					}
				}
				t.Fatalf("no %q drawn for %s", tt.word, body)
				return 0
			}
			if got, want := x(tt.runs), x(tt.whole); math.Abs(got-want) > 0.01 {
				t.Errorf("%q starts at x=%.2f in runs %s, want x=%.2f as in %q", tt.word, got, tt.runs, want, tt.whole)
			}
		})
	}
}

// pageSpans returns the text spans drawn on the first page of pdf.
func pageSpans(t *testing.T, pdf []byte) []reader.TextSpan {
	t.Helper()
	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("page 0: %v", err)
	}
	spans, err := page.TextSpans()
	if err != nil {
		t.Fatalf("text spans: %v", err)
	}
	return spans
}
