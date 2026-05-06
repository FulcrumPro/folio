// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
)

// reRectOp matches a PDF `x y w h re` operator.
var reRectOp = regexp.MustCompile(`(-?\d+(?:\.\d+)?)\s+(-?\d+(?:\.\d+)?)\s+(-?\d+(?:\.\d+)?)\s+(-?\d+(?:\.\d+)?)\s+re`)

func firstRectInStream(t *testing.T, stream []byte) (x, y, w, h float64) {
	t.Helper()
	m := reRectOp.FindStringSubmatch(string(stream))
	if m == nil {
		t.Fatalf("no `re` rectangle operator found in stream:\n%s", string(stream))
	}
	parse := func(s string) float64 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatalf("could not parse %q: %v", s, err)
		}
		return v
	}
	return parse(m[1]), parse(m[2]), parse(m[3]), parse(m[4])
}

// drawFirstLine plans a single-line paragraph and invokes its first
// PlacedBlock's Draw closure into a fresh stream at absX = block.X
// (i.e., as if the renderer placed the parent at x=0). Returns the
// stream bytes plus the block's own X for invariants relative to the
// alignment offset.
func drawFirstLine(t *testing.T, p *Paragraph, areaW float64) (stream []byte, blockX float64) {
	t.Helper()
	plan := p.PlanLayout(LayoutArea{Width: areaW, Height: 1e6})
	if len(plan.Blocks) == 0 {
		t.Fatalf("expected at least one PlacedBlock, got none")
	}
	b := plan.Blocks[0]
	s := content.NewStream()
	ctx := DrawContext{Stream: s, Page: &PageResult{}}
	b.Draw(ctx, b.X, b.Y+b.Height)
	return s.Bytes(), b.X
}

func approxEqual(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// TestParagraphBackgroundDoesNotBleedOnCenterAlignment is a regression
// test for the bug where a center-aligned paragraph background was
// shifted by the same offset as the text, leaking past the line's
// right edge. Per CSS Backgrounds & Borders L3 §2.1, the background
// paints the inline line box, not the text run, so the rect's left
// edge must stay at 0 and its width must equal the line-box width
// regardless of text alignment.
func TestParagraphBackgroundDoesNotBleedOnCenterAlignment(t *testing.T) {
	const areaW = 400
	bg := RGB(0.9, 0.9, 0.9)
	p := NewParagraph("Hi", font.Helvetica, 12).
		SetBackground(bg).
		SetAlign(AlignCenter)

	stream, blockX := drawFirstLine(t, p, areaW)
	if blockX <= 0 {
		t.Fatalf("center-aligned line should be offset from 0; got blockX=%v (test won't catch the bug otherwise)", blockX)
	}

	rectX, _, rectW, _ := firstRectInStream(t, stream)
	if !approxEqual(rectX, 0, 0.01) {
		t.Errorf("background tracked text shift: rectX=%v, expected 0 (alignment offset was %v)", rectX, blockX)
	}
	if !approxEqual(rectW, areaW, 0.01) {
		t.Errorf("background width = %v, expected line-box width %v", rectW, areaW)
	}
	if rectX+rectW > areaW+0.01 {
		t.Errorf("background right edge %v exceeds content box right edge %v", rectX+rectW, areaW)
	}
}

func TestParagraphBackgroundDoesNotBleedOnRightAlignment(t *testing.T) {
	const areaW = 400
	bg := RGB(0.9, 0.9, 0.9)
	p := NewParagraph("Hi", font.Helvetica, 12).
		SetBackground(bg).
		SetAlign(AlignRight)

	stream, blockX := drawFirstLine(t, p, areaW)
	if blockX <= 0 {
		t.Fatalf("right-aligned line should be offset from 0; got blockX=%v", blockX)
	}

	rectX, _, rectW, _ := firstRectInStream(t, stream)
	if !approxEqual(rectX, 0, 0.01) {
		t.Errorf("right-align bleed: rectX=%v, expected 0 (alignment offset was %v)", rectX, blockX)
	}
	if !approxEqual(rectW, areaW, 0.01) {
		t.Errorf("background width = %v, expected %v", rectW, areaW)
	}
}

// TestParagraphBackgroundLeftAlignmentUnchanged guards against
// regressing the left-aligned case, which was already correct.
func TestParagraphBackgroundLeftAlignmentUnchanged(t *testing.T) {
	const areaW = 400
	bg := RGB(0.9, 0.9, 0.9)
	p := NewParagraph("Hi", font.Helvetica, 12).
		SetBackground(bg).
		SetAlign(AlignLeft)

	stream, _ := drawFirstLine(t, p, areaW)
	rectX, _, rectW, _ := firstRectInStream(t, stream)
	if !approxEqual(rectX, 0, 0.01) {
		t.Errorf("left-aligned background should start at 0, got %v", rectX)
	}
	if !approxEqual(rectW, areaW, 0.01) {
		t.Errorf("left-aligned background width = %v, expected %v", rectW, areaW)
	}
}

// TestParagraphBackgroundFirstLineIndentCenterAlignment verifies that
// first-line indent and center alignment compose correctly: the
// background must start at firstIndent (not at 0, and not at
// firstIndent+alignOffset) and span (areaW - firstIndent).
func TestParagraphBackgroundFirstLineIndentCenterAlignment(t *testing.T) {
	const (
		areaW       = 400
		firstIndent = 30
	)
	bg := RGB(0.9, 0.9, 0.9)
	p := NewParagraph("Hi", font.Helvetica, 12).
		SetBackground(bg).
		SetAlign(AlignCenter)
	p.firstIndent = firstIndent

	stream, _ := drawFirstLine(t, p, areaW)
	rectX, _, rectW, _ := firstRectInStream(t, stream)

	if !approxEqual(rectX, firstIndent, 0.01) {
		t.Errorf("first-line indent dropped or shifted: rectX=%v, expected %v", rectX, firstIndent)
	}
	if !approxEqual(rectW, areaW-firstIndent, 0.01) {
		t.Errorf("first-line indent did not shrink line-box width: rectW=%v, expected %v", rectW, areaW-firstIndent)
	}
}
