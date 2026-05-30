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
// through to the black default. The neighbour stops (red, green) must
// also retain their parsed colors so that calc-position handling does
// not leak into surrounding stops.
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
	// Neighbour stops must keep their colors. Compare against the same
	// parseColor path the parser uses for the rest of the value.
	wantRed, _ := parseColor("red")
	if stops[0].Color != wantRed {
		t.Errorf("first stop color = %+v, want %+v (red)", stops[0].Color, wantRed)
	}
	wantGreen, _ := parseColor("green")
	if stops[2].Color != wantGreen {
		t.Errorf("last stop color = %+v, want %+v (green)", stops[2].Color, wantGreen)
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
// single-axis fallback. The lazy-resolution migration changes the
// mixed-unit row: x now resolves to (container - image) * 0.5 + 10px's
// point value at draw time, rather than dropping to 0 at parse time.
// The percent-only rows are unchanged in intent; this test resolves
// each axis against a 100pt "(container - image)" surrogate so the
// recovered fraction matches the pre-migration expectations.
//
// Migrated from the [2]float64 return shape. The mixed-unit row is
// rewritten to assert the new resolved-point semantics (60pt with the
// codebase's px-to-pt convention of 0.75: 100*0.5 + 10*0.75 = 57.5).
func TestParseBgPositionWithCalc(t *testing.T) {
	tests := []struct {
		input         string
		wantX         float64
		wantY         float64
		containerSize float64
		fontSize      float64
	}{
		{"calc(50% - 10%) 50%", 40, 50, 100, 0},
		{"calc(30%)", 30, 50, 100, 0},
		{"min(40%, 60%) 50%", 40, 50, 100, 0},
		{"clamp(10%, 50%, 90%) 25%", 50, 25, 100, 0},
		// Mixed-unit calc on x: now resolves lazily. 50% of 100 + 10px
		// in points = 50 + 7.5 = 57.5. Y is plain 50% = 50.
		{"calc(50% + 10px) 50%", 57.5, 50, 100, 0},
	}
	for _, tt := range tests {
		pos := parseBgPosition(tt.input)
		gotX := pos[0].Resolve(tt.containerSize, tt.fontSize)
		gotY := pos[1].Resolve(tt.containerSize, tt.fontSize)
		if math.Abs(gotX-tt.wantX) > 1e-9 || math.Abs(gotY-tt.wantY) > 1e-9 {
			t.Errorf("parseBgPosition(%q).Resolve(%v, %v) = [%v, %v], want [%v, %v]",
				tt.input, tt.containerSize, tt.fontSize, gotX, gotY, tt.wantX, tt.wantY)
		}
	}
}

// TestPercentFractionAcceptsSingleLeafCalc exercises the parser's
// single-leaf calc form (no operator). The distinction from the plain
// "<num>%" fast path inside parseLength matters: this path goes through
// the calc dispatch and produces a calcExpr leaf wrapper. The helper
// must still reduce it to the same fraction.
func TestPercentFractionAcceptsSingleLeafCalc(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"calc(50%)", 0.5},
		{"calc(0%)", 0.0},
		{"calc(100%)", 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := parseLength(tt.input)
			if l == nil {
				t.Fatalf("parseLength(%q) returned nil", tt.input)
			}
			got, ok := percentFraction(l)
			if !ok {
				t.Fatalf("percentFraction(%q): ok=false, want ok=true", tt.input)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("percentFraction(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestPercentFractionAcceptsNestedCalc pins the end-to-end shape for a
// calc expression containing a parenthesised sub-expression.
// parseCalcExpr strips a top-level (...) wrapper after the operator
// passes, so calc(2 * (50% - 10%)) reduces to the same fraction as the
// flat form calc(2 * 50% - 2 * 10%). The helper must walk into the
// nested tree and return 0.8.
func TestPercentFractionAcceptsNestedCalc(t *testing.T) {
	const input = "calc(2 * (50% - 10%))"
	l := parseLength(input)
	if l == nil {
		t.Fatalf("parseLength(%q) returned nil", input)
	}
	got, ok := percentFraction(l)
	if !ok {
		t.Fatalf("percentFraction(%q): ok=false, want ok=true", input)
	}
	if math.Abs(got-0.8) > 1e-9 {
		t.Errorf("percentFraction(%q) = %v, want 0.8", input, got)
	}
}

// TestPercentFractionAcceptsRightSideParenCalc is the mirror of
// TestPercentFractionAcceptsNestedCalc with the parenthesised sub-tree
// on the left of the operator: calc((50% - 10%) * 2). This form used
// to fail to parse because parseCalcExpr did not reset its paren-depth
// counter between the +/- scan and the * / / scan, hiding the top-level
// '*'. The fix lets parseLength produce a percent-only tree and the
// helper must reduce it to 0.8.
func TestPercentFractionAcceptsRightSideParenCalc(t *testing.T) {
	const input = "calc((50% - 10%) * 2)"
	l := parseLength(input)
	if l == nil {
		t.Fatalf("parseLength(%q) returned nil", input)
	}
	got, ok := percentFraction(l)
	if !ok {
		t.Fatalf("percentFraction(%q): ok=false, want ok=true", input)
	}
	if math.Abs(got-0.8) > 1e-9 {
		t.Errorf("percentFraction(%q) = %v, want 0.8", input, got)
	}
}

// TestPercentFractionRejectsBareNumberAsPx pins the bare-number-as-px
// convention from parsePlainLength: parseLength("1.5") returns a
// cssLength tagged with Unit "px", not "num". percentFraction must
// reject it because px is a length unit, not a fraction component.
// This is a distinct code path from the explicit "10px" test in
// TestPercentFractionRejectsPureLength.
func TestPercentFractionRejectsBareNumberAsPx(t *testing.T) {
	l := parseLength("1.5")
	switch {
	case l == nil:
		t.Fatalf(`parseLength("1.5") returned nil`)
	case l.Unit != "px":
		t.Fatalf(`parseLength("1.5").Unit = %q, want "px" (bare-number convention)`, l.Unit)
	}
	got, ok := percentFraction(l)
	if ok {
		t.Errorf(`percentFraction("1.5") = (%v, true), want (_, false)`, got)
	}
}

// TestPercentFractionNilSafe guards the nil-check at the helper's entry.
// Position parsers may hand a nil cssLength to percentFraction when an
// earlier parse step failed; the helper must not panic.
func TestPercentFractionNilSafe(t *testing.T) {
	got, ok := percentFraction(nil)
	if ok {
		t.Errorf("percentFraction(nil) = (%v, true), want (0, false)", got)
	}
	if got != 0 {
		t.Errorf("percentFraction(nil) value = %v, want 0", got)
	}
}

// TestPercentFractionNegativeResult pins pass-through behaviour for
// negative fractions. Per CSS Images L3 §3.4.3, negative gradient stop
// positions are spec-valid (they extrapolate the gradient). A future
// "clamp to [0, 1]" decision should be intentional, not an accidental
// side-effect of the helper.
func TestPercentFractionNegativeResult(t *testing.T) {
	const input = "calc(0% - 10%)"
	l := parseLength(input)
	if l == nil {
		t.Fatalf("parseLength(%q) returned nil", input)
	}
	got, ok := percentFraction(l)
	if !ok {
		t.Fatalf("percentFraction(%q): ok=false, want ok=true", input)
	}
	if math.Abs(got-(-0.1)) > 1e-9 {
		t.Errorf("percentFraction(%q) = %v, want -0.1", input, got)
	}
}

// TestParseGradientStopsLengthPositionDrops pins the documented plain-
// length limitation: "blue 100px" cannot be reduced to a fraction
// without the gradient line length. The token is rejected as a
// position, the whole field is treated as a color, parseColor fails on
// "blue 100px", and the stop falls back to default Position=0. See the
// existing TestParseGradientStopsMixedUnitCalcFallsBack reproducer-pin
// for the same shape with mixed-unit calc.
func TestParseGradientStopsLengthPositionDrops(t *testing.T) {
	parts := []string{"red", "blue 100px", "green"}
	stops := parseGradientStops(parts)
	if len(stops) != 3 {
		t.Fatalf("got %d stops, want 3", len(stops))
	}
	if stops[1].Position != 0 {
		t.Errorf("middle stop position = %v, want 0 (default for unparseable)",
			stops[1].Position)
	}
}

// TestParseBgPositionSingleAxisY exercises the y-axis branch of
// parseBgPosition with a calc value. The existing TestParseBgPositionWithCalc
// only covers calc on the x axis; this test pairs a plain percent x
// with a single-leaf calc y so the calc dispatch runs on parts[1].
// Migrated to the resolved-points shape: a 100pt container makes the
// recovered fractions match the pre-migration expectations.
func TestParseBgPositionSingleAxisY(t *testing.T) {
	pos := parseBgPosition("50% calc(30%)")
	gotX := pos[0].Resolve(100, 0) / 100
	gotY := pos[1].Resolve(100, 0) / 100
	if math.Abs(gotX-0.5) > 1e-9 || math.Abs(gotY-0.3) > 1e-9 {
		t.Errorf("parseBgPosition(%q) = [%v, %v], want [0.5, 0.3]",
			"50% calc(30%)", gotX, gotY)
	}
}

// TestParseBgPositionKeywordPlusCalc pins the keyword + calc interplay
// on both axes. The keyword resolves through parseAxis's switch to a
// percent constant (bgPosZero / bgPosHalf / bgPosFull), the calc
// resolves through parseLength, and the two compose positionally.
// Migrated from the [2]float64 return shape; a 100pt container surrogate
// recovers the original fractions.
func TestParseBgPositionKeywordPlusCalc(t *testing.T) {
	tests := []struct {
		input string
		wantX float64
		wantY float64
	}{
		{"left calc(20%)", 0, 0.2},
		{"center calc(30%)", 0.5, 0.3},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pos := parseBgPosition(tt.input)
			gotX := pos[0].Resolve(100, 0) / 100
			gotY := pos[1].Resolve(100, 0) / 100
			if math.Abs(gotX-tt.wantX) > 1e-9 || math.Abs(gotY-tt.wantY) > 1e-9 {
				t.Errorf("parseBgPosition(%q) = [%v, %v], want [%v, %v]",
					tt.input, gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
}

// TestParseBgPositionBothAxesMixedUnit exercises mixed-unit calc on
// both axes simultaneously. After the lazy-resolution migration both
// axes resolve at draw time: 50% of a 100pt (container - image) box
// plus / minus 10px (= 7.5pt). Migrated from the
// "both axes fall back to 0" assertion that captured the old behaviour.
func TestParseBgPositionBothAxesMixedUnit(t *testing.T) {
	pos := parseBgPosition("calc(50% + 10px) calc(50% - 10px)")
	gotX := pos[0].Resolve(100, 0)
	gotY := pos[1].Resolve(100, 0)
	if math.Abs(gotX-57.5) > 1e-9 || math.Abs(gotY-42.5) > 1e-9 {
		t.Errorf("parseBgPosition(%q).Resolve(100, 0) = [%v, %v], want [57.5, 42.5]",
			"calc(50% + 10px) calc(50% - 10px)", gotX, gotY)
	}
}

// TestParseBgPositionLengthDrops formerly pinned the plain-length spec
// gap. The gap is closed by lazy resolution: the x-axis "100px" now
// resolves to 75pt (px-to-pt 0.75) at draw time and the y-axis "50%"
// resolves to half of (container - image). The test name is kept for
// git-blame continuity but the assertion now confirms the closure.
func TestParseBgPositionLengthDrops(t *testing.T) {
	pos := parseBgPosition("100px 50%")
	gotX := pos[0].Resolve(200, 0)
	gotY := pos[1].Resolve(100, 0)
	if math.Abs(gotX-75) > 1e-9 || math.Abs(gotY-50) > 1e-9 {
		t.Errorf("parseBgPosition(%q) = [%v, %v], want [75, 50]",
			"100px 50%", gotX, gotY)
	}
}
