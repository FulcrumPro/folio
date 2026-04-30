// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

const measureFloatEpsilon = 1e-9

func measureNearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < measureFloatEpsilon
}

func TestMeasureLinesSingleLine(t *testing.T) {
	p := NewParagraph("Hello World", font.Helvetica, 12)
	if got := p.MeasureLines(500); got != 1 {
		t.Errorf("MeasureLines(500) = %d, want 1", got)
	}
}

func TestMeasureLinesWraps(t *testing.T) {
	p := NewParagraph("Hello World", font.Helvetica, 12)
	if got := p.MeasureLines(40); got != 2 {
		t.Errorf("MeasureLines(40) = %d, want 2", got)
	}
}

func TestMeasureLinesEmpty(t *testing.T) {
	p := NewParagraph("", font.Helvetica, 12)
	if got := p.MeasureLines(500); got != 0 {
		t.Errorf("MeasureLines empty = %d, want 0", got)
	}
}

func TestMeasureLinesAgreesWithLayout(t *testing.T) {
	// MeasureLines must equal len(Layout) for any width — they share
	// the wrap path. This guards against future wrap-path divergence.
	text := strings.Repeat("Acme Corp is renewing their Pro tier. ", 12)
	p := NewParagraph(text, font.Helvetica, 12)
	for _, w := range []float64{60, 120, 240, 480} {
		got := p.MeasureLines(w)
		want := len(p.Layout(w))
		if got != want {
			t.Errorf("width=%v: MeasureLines=%d, len(Layout)=%d", w, got, want)
		}
	}
}

func TestMeasureLinesForcedLineBreak(t *testing.T) {
	// Forced \n in source must count as line breaks even if the line
	// fits the width (so callers can clamp on "rendered lines",
	// including manually-broken ones).
	p := NewParagraph("alpha\nbeta\ngamma", font.Helvetica, 12)
	if got := p.MeasureLines(500); got != 3 {
		t.Errorf("MeasureLines with forced \\n = %d, want 3", got)
	}
}

func TestMeasureLinesDoesNotMutate(t *testing.T) {
	// Calling MeasureLines twice with different widths must not leave
	// state behind that affects the second call.
	text := strings.Repeat("alpha beta gamma delta ", 6)
	p := NewParagraph(text, font.Helvetica, 12)
	wide := p.MeasureLines(500)
	narrow := p.MeasureLines(80)
	again := p.MeasureLines(500)
	if wide != again {
		t.Errorf("MeasureLines is non-deterministic: %d vs %d at same width", wide, again)
	}
	if narrow <= wide {
		t.Errorf("narrow width should produce more lines: wide=%d, narrow=%d", wide, narrow)
	}
}

func TestMeasureHeightSingleLine(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 10)
	got := p.MeasureHeight(500)
	// Default leading is 1.2 → 12pt per line.
	want := 12.0
	if !measureNearlyEqual(got, want) {
		t.Errorf("MeasureHeight = %v, want %v", got, want)
	}
}

func TestMeasureHeightCustomLeading(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 10).SetLeading(1.5)
	got := p.MeasureHeight(500)
	if !measureNearlyEqual(got, 15.0) {
		t.Errorf("MeasureHeight with leading=1.5 = %v, want 15", got)
	}
}

func TestMeasureHeightMultiLine(t *testing.T) {
	p := NewParagraph("Hello World", font.Helvetica, 12)
	got := p.MeasureHeight(40) // forces 2 lines, default leading 1.2 → 14.4 each
	want := 2 * 14.4
	if !measureNearlyEqual(got, want) {
		t.Errorf("MeasureHeight wrap = %v, want %v", got, want)
	}
}

func TestMeasureHeightExcludesSpacing(t *testing.T) {
	// SpaceBefore/SpaceAfter are caller-owned; MeasureHeight reports
	// only the rendered line stack so callers can compose with their
	// own pagination math.
	p := NewParagraph("Hello", font.Helvetica, 10).
		SetSpaceBefore(20).
		SetSpaceAfter(30)
	if got := p.MeasureHeight(500); !measureNearlyEqual(got, 12.0) {
		t.Errorf("MeasureHeight with spaceBefore/After = %v, want 12 (excludes spacing)", got)
	}
}

func TestMeasureHeightEmpty(t *testing.T) {
	p := NewParagraph("", font.Helvetica, 12)
	if got := p.MeasureHeight(500); got != 0 {
		t.Errorf("MeasureHeight empty = %v, want 0", got)
	}
}

func TestMeasureHeightAgreesWithLayout(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta ", 8)
	p := NewParagraph(text, font.Helvetica, 12).SetLeading(1.5)
	for _, w := range []float64{60, 120, 240, 480} {
		got := p.MeasureHeight(w)
		want := 0.0
		for _, line := range p.Layout(w) {
			want += line.Height
		}
		if !measureNearlyEqual(got, want) {
			t.Errorf("width=%v: MeasureHeight=%v, sum(Layout heights)=%v", w, got, want)
		}
	}
}

func TestMeasureLinesCJK(t *testing.T) {
	// CJK ideographs break per-character. MeasureLines should count
	// the wrapped lines correctly even though there are no inter-word
	// spaces.
	text := strings.Repeat("中文文本", 20)
	p := NewParagraph(text, font.Helvetica, 12)
	if got := p.MeasureLines(60); got <= 1 {
		t.Errorf("CJK MeasureLines at narrow width = %d, want >1", got)
	}
}

func TestMeasureLinesRTL(t *testing.T) {
	// Hebrew paragraph — measurement is direction-agnostic; just
	// verify wrapping happens correctly for a real RTL run.
	p := NewParagraph(strings.Repeat("שלום עולם ", 8), font.Helvetica, 12).
		SetDirection(DirectionRTL)
	if got := p.MeasureLines(60); got <= 1 {
		t.Errorf("RTL MeasureLines at narrow width = %d, want >1", got)
	}
}
