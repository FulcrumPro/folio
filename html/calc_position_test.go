// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"
)

// TestPercentFractionAcceptsPercentOnlyTrees covers the restricted-calc
// branch shared by parseGradientStops and parseBgPosition: any
// calc/min/max/clamp tree whose leaves are all percent or dimensionless
// must reduce to a [0..1] fraction without needing a resolution context.
func TestPercentFractionAcceptsPercentOnlyTrees(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"50%", 0.5},
		{"0%", 0.0},
		{"100%", 1.0},
		{"calc(50% - 10%)", 0.4},
		{"calc(50% + 30%)", 0.8},
		{"calc(50% * 2)", 1.0},
		{"calc(60% / 2)", 0.3},
		{"min(40%, 60%)", 0.4},
		{"max(0%, 30%)", 0.3},
		{"clamp(10%, 50%, 90%)", 0.5},
		{"clamp(10%, 5%, 90%)", 0.1},  // preferred below min -> clamp up
		{"clamp(10%, 95%, 90%)", 0.9}, // preferred above max -> clamp down
	}
	for _, tt := range tests {
		l := parseLength(tt.input)
		if l == nil {
			t.Errorf("parseLength(%q) returned nil", tt.input)
			continue
		}
		got, ok := percentFraction(l)
		if !ok {
			t.Errorf("percentFraction(%q): ok=false, want ok=true", tt.input)
			continue
		}
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("percentFraction(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestPercentFractionRejectsMixedUnit confirms that any leaf carrying a
// length unit poisons the whole tree — the position parsers cannot
// reduce mixed-unit calc to a fraction without the resolution dimension
// (gradient line length or background box), so the helper must reject.
func TestPercentFractionRejectsMixedUnit(t *testing.T) {
	tests := []string{
		"calc(50% + 10px)",
		"calc(50% - 10px)",
		"calc(1em + 50%)",
		"calc(50% + 1pt)",
		"min(50%, 10px)",
		"max(10px, 50%)",
		"clamp(10%, 50%, 100px)",
	}
	for _, in := range tests {
		l := parseLength(in)
		if l == nil {
			t.Errorf("parseLength(%q) returned nil", in)
			continue
		}
		got, ok := percentFraction(l)
		if ok {
			t.Errorf("percentFraction(%q) = (%v, true), want (_, false)", in, got)
		}
	}
}

// TestPercentFractionRejectsPureLength confirms a plain length without
// percent is also rejected — it's a valid cssLength but not a fraction.
func TestPercentFractionRejectsPureLength(t *testing.T) {
	tests := []string{"10px", "2em", "12pt", "1rem", "calc(10px + 5px)"}
	for _, in := range tests {
		l := parseLength(in)
		if l == nil {
			t.Errorf("parseLength(%q) returned nil", in)
			continue
		}
		got, ok := percentFraction(l)
		if ok {
			t.Errorf("percentFraction(%q) = (%v, true), want (_, false)", in, got)
		}
	}
}

// TestParseGradientStopsWithCalcPosition drives parseGradientStops with
// the parts list produced by parseLinearGradient for
// "linear-gradient(red, blue calc(50% - 10%), green)". The middle stop
// must land at 0.4 with a non-zero blue color, not be dropped or fall
// through to the black default.
func TestParseGradientStopsWithCalcPosition(t *testing.T) {
	parts := []string{"red", "blue calc(50% - 10%)", "green"}
	stops := parseGradientStops(parts)
	if len(stops) != 3 {
		t.Fatalf("got %d stops, want 3", len(stops))
	}
	if math.Abs(stops[1].Position-0.4) > 1e-9 {
		t.Errorf("middle stop position = %v, want 0.4", stops[1].Position)
	}
	// Blue is sRGB (0, 0, 1). Confirm parseColor ran on "blue" only.
	if stops[1].Color.R != 0 || stops[1].Color.G != 0 || stops[1].Color.B == 0 {
		t.Errorf("middle stop color = %+v, want blue (R=0 G=0 B=1)", stops[1].Color)
	}
}

// TestParseGradientStopsPlainPercentStillWorks guards the
// fast-path: literal "<num>%" positions must keep working after the
// tokenizer swap.
func TestParseGradientStopsPlainPercentStillWorks(t *testing.T) {
	parts := []string{"#ff0000 0%", "#0000ff 100%"}
	stops := parseGradientStops(parts)
	if len(stops) != 2 {
		t.Fatalf("got %d stops, want 2", len(stops))
	}
	if stops[0].Position != 0 || stops[1].Position != 1 {
		t.Errorf("positions = [%v, %v], want [0, 1]",
			stops[0].Position, stops[1].Position)
	}
}

// TestParseGradientStopsMixedUnitCalcFallsBack confirms the documented
// limitation: mixed-unit calc cannot be reduced to a fraction without
// the gradient line length, so the whole field is treated as a color.
// parseColor then fails on "blue calc(50% + 10px)" and the stop's color
// is left zero. This matches "rejected, not resolved" — the renderer's
// existing behaviour for unparseable stops.
func TestParseGradientStopsMixedUnitCalcFallsBack(t *testing.T) {
	parts := []string{"blue calc(50% + 10px)"}
	stops := parseGradientStops(parts)
	if len(stops) != 1 {
		t.Fatalf("got %d stops, want 1", len(stops))
	}
	if stops[0].Position != 0 {
		t.Errorf("position = %v, want 0 (default for unparseable)",
			stops[0].Position)
	}
}

// TestParseBgPositionWithCalc covers the percent-only calc path and the
// single-axis fallback. Mixed-unit calc must fall back to the existing
// "unparseable axis -> default" behaviour.
func TestParseBgPositionWithCalc(t *testing.T) {
	tests := []struct {
		input string
		wantX float64
		wantY float64
	}{
		{"calc(50% - 10%) 50%", 0.4, 0.5},
		{"calc(30%)", 0.3, 0.5},
		{"min(40%, 60%) 50%", 0.4, 0.5},
		{"clamp(10%, 50%, 90%) 25%", 0.5, 0.25},
		// Mixed-unit calc on x: x falls back to 0 (existing
		// behaviour for unparseable), y still resolves.
		{"calc(50% + 10px) 50%", 0, 0.5},
	}
	for _, tt := range tests {
		pos := parseBgPosition(tt.input)
		if math.Abs(pos[0]-tt.wantX) > 1e-9 || math.Abs(pos[1]-tt.wantY) > 1e-9 {
			t.Errorf("parseBgPosition(%q) = [%v, %v], want [%v, %v]",
				tt.input, pos[0], pos[1], tt.wantX, tt.wantY)
		}
	}
}
