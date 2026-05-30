// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"
)

// Tests in this file cover the lazy-resolution behaviour added to close
// the background-position half of issue #266. parseBgPosition now
// returns layout.ResolvableLength values that resolve at draw time
// against (container - image) on each axis. Plain lengths and
// mixed-unit calc — the long-standing gaps — are now valid inputs.
// The gradient-stops half of the parent issue is tracked separately in
// issue #318 and is not exercised here.

// TestBgPositionPlainLength pins the plain-length axis path. 100px and
// 50px must resolve to their points-equivalent (px to pt is 0.75 in
// the codebase: 100px = 75pt, 50px = 37.5pt) on both axes regardless
// of the container dimension. Plain lengths are raw offsets; the
// container argument is unused at the leaf.
func TestBgPositionPlainLength(t *testing.T) {
	pos := parseBgPosition("100px 50px")
	for _, container := range []float64{50, 200, 1000} {
		gotX := pos[0].Resolve(container, 0)
		gotY := pos[1].Resolve(container, 0)
		if math.Abs(gotX-75) > 1e-9 || math.Abs(gotY-37.5) > 1e-9 {
			t.Errorf("Resolve(container=%v) = [%v, %v], want [75, 37.5]",
				container, gotX, gotY)
		}
	}
}

// TestBgPositionPlainLengthEm exercises em-relative plain lengths.
// fontSize=16 feeds the em multiplier directly: 2em = 32pt, 1em = 16pt.
func TestBgPositionPlainLengthEm(t *testing.T) {
	pos := parseBgPosition("2em 1em")
	gotX := pos[0].Resolve(100, 16)
	gotY := pos[1].Resolve(100, 16)
	if math.Abs(gotX-32) > 1e-9 || math.Abs(gotY-16) > 1e-9 {
		t.Errorf("Resolve = [%v, %v], want [32, 16]", gotX, gotY)
	}
}

// TestBgPositionMixedUnitCalcX pins the mixed-unit calc path closed by
// the migration. calc(50% + 10px) against a (container - image) box of
// 100pt resolves to 50%*100 + 10px*0.75 = 50 + 7.5 = 57.5pt. The y
// axis stays plain percent.
func TestBgPositionMixedUnitCalcX(t *testing.T) {
	pos := parseBgPosition("calc(50% + 10px) 50%")
	gotX := pos[0].Resolve(100, 0)
	gotY := pos[1].Resolve(100, 0)
	if math.Abs(gotX-57.5) > 1e-9 || math.Abs(gotY-50) > 1e-9 {
		t.Errorf("Resolve = [%v, %v], want [57.5, 50]", gotX, gotY)
	}
}

// TestBgPositionPercentY pins plain percent on both axes with
// dissimilar container sizes for x and y. 50% of a 100pt (container-
// image) x box = 50; 25% of a 50pt (container - image) y box = 12.5.
func TestBgPositionPercentY(t *testing.T) {
	pos := parseBgPosition("50% 25%")
	gotX := pos[0].Resolve(100, 0)
	gotY := pos[1].Resolve(50, 0)
	if math.Abs(gotX-50) > 1e-9 || math.Abs(gotY-12.5) > 1e-9 {
		t.Errorf("Resolve = [%v, %v], want [50, 12.5]", gotX, gotY)
	}
}

// TestBgPositionKeywordPlusCalc pins the keyword + calc compose path
// after the lazy migration. "left" maps to the percent constant 0%, so
// it resolves to 0 regardless of container. calc(30%) resolves
// against the y container.
func TestBgPositionKeywordPlusCalc(t *testing.T) {
	pos := parseBgPosition("left calc(30%)")
	gotX := pos[0].Resolve(123, 0)
	gotY := pos[1].Resolve(100, 0)
	if math.Abs(gotX-0) > 1e-9 || math.Abs(gotY-30) > 1e-9 {
		t.Errorf("Resolve = [%v, %v], want [0, 30]", gotX, gotY)
	}
}

// TestBgPositionSingleAxisLength pins the single-axis default for a
// plain length input. "100px" targets the x axis (it is not "top" or
// "bottom"); the y axis defaults to the bgPosHalf percent constant
// (50%). Resolution against a 100pt y (container - image) yields 50.
func TestBgPositionSingleAxisLength(t *testing.T) {
	pos := parseBgPosition("100px")
	gotX := pos[0].Resolve(200, 0)
	gotY := pos[1].Resolve(100, 0)
	if math.Abs(gotX-75) > 1e-9 || math.Abs(gotY-50) > 1e-9 {
		t.Errorf("Resolve = [%v, %v], want [75, 50]", gotX, gotY)
	}
}

// TestBgPositionUnitlessNumberAsPx pins the folio convention from
// parsePlainLength: a bare number with no unit is tagged with Unit
// "px". Resolution applies the px-to-pt 0.75 factor. The single-axis
// fallback applies as above: y defaults to bgPosHalf.
func TestBgPositionUnitlessNumberAsPx(t *testing.T) {
	pos := parseBgPosition("100")
	gotX := pos[0].Resolve(200, 0)
	gotY := pos[1].Resolve(100, 0)
	if math.Abs(gotX-75) > 1e-9 || math.Abs(gotY-50) > 1e-9 {
		t.Errorf("Resolve = [%v, %v], want [75, 50]", gotX, gotY)
	}
}

// TestBgPositionInvalidFallsBack confirms the parser preserves its
// "unrecognised input -> default" semantics after the migration. A
// garbage token drops both axes to the bgPosZero constant, which
// resolves to 0 regardless of the container dimension.
func TestBgPositionInvalidFallsBack(t *testing.T) {
	pos := parseBgPosition("garbage")
	gotX := pos[0].Resolve(200, 0)
	gotY := pos[1].Resolve(100, 0)
	if gotX != 0 || gotY != 0 {
		t.Errorf("Resolve = [%v, %v], want [0, 0]", gotX, gotY)
	}
}
