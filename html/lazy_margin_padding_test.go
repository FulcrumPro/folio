// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"
)

// TestParseBoxSideBothReturnsBothForms is the parser-side contract
// for the #269 Phase 1 sibling-field migration. Every margin/padding
// Apply must produce BOTH:
//
//   - a legacy float64 resolved against zero (the existing 0pt-on-
//     percent behaviour) for back-compat with unmigrated consumers
//   - an unresolved *cssLength that preserves the percent / calc tree
//     for layout-time resolution against the container width
//
// "auto" / unparseable input still yields (0, nil) so callers can
// branch on the *cssLength being nil.
func TestParseBoxSideBothReturnsBothForms(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		fontSize   float64
		wantLegacy float64
		wantUnit   string  // expected cssLength.Unit; "" means nil expected
		wantValue  float64 // for non-nil, expected cssLength.Value
	}{
		{"plain points", "10pt", 12, 10, "pt", 10},
		// 16px → 12pt at 0.75 px/pt.
		{"plain pixels", "16px", 12, 12, "px", 16},
		// percent: legacy resolves against 0 (the bug); sibling
		// retains the 50% form for layout-time resolution.
		{"percent", "50%", 12, 0, "%", 50},
		// em on the legacy path resolves against fontSize, so 1em
		// at 12pt → 12pt. The sibling carries Unit "em" so a future
		// consumer can re-resolve.
		{"em", "1.5em", 12, 18, "em", 1.5},
		// Unparseable inputs return (0, nil).
		{"empty", "", 12, 0, "", 0},
		{"auto keyword (parseLength rejects)", "auto", 12, 0, "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotLegacy, gotLength := parseBoxSideBoth(tc.input, tc.fontSize)
			if math.Abs(gotLegacy-tc.wantLegacy) > 0.001 {
				t.Errorf("legacy float64 = %v, want %v", gotLegacy, tc.wantLegacy)
			}
			if tc.wantUnit == "" {
				if gotLength != nil {
					t.Errorf("cssLength = %+v, want nil", gotLength)
				}
				return
			}
			if gotLength == nil {
				t.Fatalf("cssLength = nil, want Unit=%q Value=%v", tc.wantUnit, tc.wantValue)
			}
			if gotLength.Unit != tc.wantUnit {
				t.Errorf("cssLength.Unit = %q, want %q", gotLength.Unit, tc.wantUnit)
			}
			if math.Abs(gotLength.Value-tc.wantValue) > 0.001 {
				t.Errorf("cssLength.Value = %v, want %v", gotLength.Value, tc.wantValue)
			}
		})
	}
}

// TestMarginTopAtResolvesPercentAgainstContainer is the central Phase 1
// claim: when the parser stored a *cssLength sibling, MarginTopAt
// resolves percent correctly against the container width — closing the
// silent 0pt bug at the helper level. Phase 2 migrates consumers to
// the helper so the bug closes end-to-end.
//
// Each subtest synthesizes a computedStyle as the parser would
// produce it, then asserts the helper's resolution. All eight
// sides (4 margin + 4 padding) follow the same shape; the test
// table covers one of each plus a few targeted edge cases.
func TestMarginTopAtResolvesPercentAgainstContainer(t *testing.T) {
	type sideHelper func(*computedStyle, float64) float64
	type sideSetter func(*computedStyle, *cssLength)

	apply := []struct {
		name   string
		set    sideSetter
		helper sideHelper
	}{
		{"MarginTop", func(s *computedStyle, l *cssLength) { s.MarginTopLength = l }, (*computedStyle).MarginTopAt},
		{"MarginRight", func(s *computedStyle, l *cssLength) { s.MarginRightLength = l }, (*computedStyle).MarginRightAt},
		{"MarginBottom", func(s *computedStyle, l *cssLength) { s.MarginBottomLength = l }, (*computedStyle).MarginBottomAt},
		{"MarginLeft", func(s *computedStyle, l *cssLength) { s.MarginLeftLength = l }, (*computedStyle).MarginLeftAt},
		{"PaddingTop", func(s *computedStyle, l *cssLength) { s.PaddingTopLength = l }, (*computedStyle).PaddingTopAt},
		{"PaddingRight", func(s *computedStyle, l *cssLength) { s.PaddingRightLength = l }, (*computedStyle).PaddingRightAt},
		{"PaddingBottom", func(s *computedStyle, l *cssLength) { s.PaddingBottomLength = l }, (*computedStyle).PaddingBottomAt},
		{"PaddingLeft", func(s *computedStyle, l *cssLength) { s.PaddingLeftLength = l }, (*computedStyle).PaddingLeftAt},
	}
	cases := []struct {
		name           string
		value          string
		containerWidth float64
		fontSize       float64
		want           float64
	}{
		// The bug-closing case: 50% of a 200pt container = 100pt.
		// Legacy parseBoxSide would have stored 0pt.
		{"percent against container", "50%", 200, 12, 100},
		// calc(10% + 5px) at 200pt container, 12pt fontSize:
		// 10% of 200 = 20pt; 5px = 3.75pt; total 23.75pt.
		{"calc mixed", "calc(10% + 5px)", 200, 12, 23.75},
		// Plain px: container width irrelevant.
		{"plain px ignores container", "16px", 200, 12, 12},
		// em: relative to fontSize, not container.
		{"em ignores container", "2em", 200, 12, 24},
	}
	for _, h := range apply {
		for _, tc := range cases {
			t.Run(h.name+"/"+tc.name, func(t *testing.T) {
				s := &computedStyle{FontSize: tc.fontSize}
				_, length := parseBoxSideBoth(tc.value, tc.fontSize)
				if length == nil {
					t.Fatalf("parseBoxSideBoth returned nil cssLength for %q", tc.value)
				}
				h.set(s, length)
				got := h.helper(s, tc.containerWidth)
				if math.Abs(got-tc.want) > 0.001 {
					t.Errorf("got %v, want %v", got, tc.want)
				}
			})
		}
	}
}

// TestMarginTopAtFallsBackToLegacyWhenLengthAbsent guards the Phase 1
// migration invariant: a computedStyle that has only the legacy
// float64 populated (e.g. heading default margins set in
// converter_style.go, page-level margins from html/page.go, any
// future code path that bypasses the Apply registry) must still
// return the legacy value through the helper. Without this fallback
// Phase 2 consumers would read zero for every unmigrated setter.
func TestMarginTopAtFallsBackToLegacyWhenLengthAbsent(t *testing.T) {
	s := &computedStyle{
		FontSize:  12,
		MarginTop: 42, // legacy float64 set directly
		// MarginTopLength deliberately left nil.
	}
	if got := s.MarginTopAt(100); math.Abs(got-42) > 0.001 {
		t.Errorf("helper did not fall back to legacy MarginTop: got %v, want 42", got)
	}
}

// TestConvertPopulatesLengthSiblings verifies the end-to-end wire from
// the CSS Apply registry: a margin / padding declaration in a
// stylesheet should populate BOTH the legacy float64 AND the
// *cssLength sibling on the resulting computedStyle. Without this,
// Phase 2 consumer migrations would have nothing to read.
//
// The test reaches into the converter's internal style-application
// path (computeStyle on a tiny synthetic node) rather than driving
// the full HTML → PDF pipeline, because computedStyle is a package-
// internal type not exposed via ConvertResult.
func TestConvertPopulatesLengthSiblings(t *testing.T) {
	tests := []struct {
		name  string
		decls []cssDecl
		check func(*testing.T, *computedStyle)
	}{
		{
			name: "margin shorthand four-value populates all four lengths",
			decls: []cssDecl{
				{property: "margin", value: "10% 20pt 30% 40px"},
			},
			check: func(t *testing.T, s *computedStyle) {
				for _, side := range []struct {
					name   string
					length *cssLength
				}{
					{"MarginTopLength", s.MarginTopLength},
					{"MarginRightLength", s.MarginRightLength},
					{"MarginBottomLength", s.MarginBottomLength},
					{"MarginLeftLength", s.MarginLeftLength},
				} {
					if side.length == nil {
						t.Errorf("%s nil; shorthand did not populate sibling", side.name)
					}
				}
			},
		},
		{
			name: "margin-top individual property populates sibling",
			decls: []cssDecl{
				{property: "margin-top", value: "calc(10% + 5px)"},
			},
			check: func(t *testing.T, s *computedStyle) {
				if s.MarginTopLength == nil {
					t.Fatal("MarginTopLength nil")
				}
				if s.MarginTopLength.calc == nil {
					t.Error("MarginTopLength.calc nil; calc expression not preserved")
				}
			},
		},
		{
			name: "padding shorthand populates all four padding lengths",
			decls: []cssDecl{
				{property: "padding", value: "5% 10pt"},
			},
			check: func(t *testing.T, s *computedStyle) {
				for _, side := range []struct {
					name   string
					length *cssLength
				}{
					{"PaddingTopLength", s.PaddingTopLength},
					{"PaddingRightLength", s.PaddingRightLength},
					{"PaddingBottomLength", s.PaddingBottomLength},
					{"PaddingLeftLength", s.PaddingLeftLength},
				} {
					if side.length == nil {
						t.Errorf("%s nil", side.name)
					}
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &converter{opts: (&Options{}).defaults()}
			style := defaultStyle()
			style.FontSize = 12
			for _, d := range tc.decls {
				c.applyProperty(d.property, d.value, &style)
			}
			tc.check(t, &style)
		})
	}
}
