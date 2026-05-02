// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestParseTextDecorationOverline pins the overline parsing path
// per CSS Text Decoration L4 §3.1. Pre-fix `parseTextDecoration`
// returned a bitset with only Underline / Strikethrough flags;
// `text-decoration: overline` silently produced 0 (DecorationNone).
func TestParseTextDecorationOverline(t *testing.T) {
	tests := []struct {
		value string
		want  layout.TextDecoration
	}{
		// Single keywords.
		{"overline", layout.DecorationOverline},
		{"underline", layout.DecorationUnderline},
		{"line-through", layout.DecorationStrikethrough},
		{"none", layout.DecorationNone},

		// Multi-flag combinations.
		{"underline overline", layout.DecorationUnderline | layout.DecorationOverline},
		{"overline line-through", layout.DecorationOverline | layout.DecorationStrikethrough},
		{"underline overline line-through", layout.DecorationUnderline | layout.DecorationOverline | layout.DecorationStrikethrough},

		// Order-independence and case-insensitivity.
		{"OVERLINE", layout.DecorationOverline},
		{"  overline  ", layout.DecorationOverline},
		{"line-through underline overline", layout.DecorationUnderline | layout.DecorationOverline | layout.DecorationStrikethrough},

		// `blink` is recognised but a no-op (PDFs are static).
		{"overline blink", layout.DecorationOverline},
		{"blink", layout.DecorationNone},

		// Unknown / empty.
		{"", layout.DecorationNone},
		{"banana", layout.DecorationNone},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			got := parseTextDecoration(tc.value)
			if got != tc.want {
				t.Errorf("parseTextDecoration(%q) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

// TestOverlineRoundTripsThroughConvert verifies the property survives
// the full HTML → computedStyle → TextRun → Word.Decoration pipeline.
// The Word.Decoration bitset is what the renderer's drawDecorations
// switches on, so reaching it intact is the integration contract.
func TestOverlineRoundTripsThroughConvert(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		wantFlag layout.TextDecoration
	}{
		{
			name:     "explicit overline",
			html:     `<p style="text-decoration: overline">x</p>`,
			wantFlag: layout.DecorationOverline,
		},
		{
			name:     "underline + overline combined",
			html:     `<p style="text-decoration: underline overline">x</p>`,
			wantFlag: layout.DecorationUnderline | layout.DecorationOverline,
		},
		{
			name:     "all three lines",
			html:     `<p style="text-decoration: underline overline line-through">x</p>`,
			wantFlag: layout.DecorationUnderline | layout.DecorationOverline | layout.DecorationStrikethrough,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			elems, err := Convert(tc.html, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(elems) == 0 {
				t.Fatal("no elements")
			}
			p, ok := elems[0].(*layout.Paragraph)
			if !ok {
				t.Fatalf("got %T, want *layout.Paragraph", elems[0])
			}
			runs := p.Runs()
			if len(runs) == 0 {
				t.Fatal("paragraph has no runs")
			}
			got := runs[0].Decoration
			if got&tc.wantFlag != tc.wantFlag {
				t.Errorf("run decoration = %d, missing wanted bits %d (full want %d)", got, tc.wantFlag&^got, tc.wantFlag)
			}
		})
	}
}
