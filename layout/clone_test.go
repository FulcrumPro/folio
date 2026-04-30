// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// TestInlineElementSurvivesPageSplit verifies that an inline element
// appearing in the overflow portion of a paragraph survives the page
// split. Before this fix, wordToRun did not copy Word.InlineBlock to
// TextRun.InlineElement, so any inline image in the second half was
// silently lost.
func TestInlineElementSurvivesPageSplit(t *testing.T) {
	el := &fixedElement{width: 20, height: 16}
	// Long prefix forces the inline element into the overflow.
	prefix := strings.Repeat("alpha beta gamma delta ", 16)
	p := NewStyledParagraph(
		NewRun(prefix, font.Helvetica, 12),
		RunInline(el),
		NewRun(" tail tail tail", font.Helvetica, 12),
	)

	plan := p.PlanLayout(LayoutArea{Width: 80, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow, ok := plan.Overflow.(*Paragraph)
	if !ok {
		t.Fatalf("overflow type = %T, want *Paragraph", plan.Overflow)
	}

	// Walk the overflow paragraph's runs and assert the inline element
	// is present somewhere — same pointer as the original, not a copy.
	found := false
	for _, run := range overflow.Runs() {
		if run.InlineElement == el {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("inline element lost across page split; overflow runs = %d", len(overflow.Runs()))
	}
}

// TestInlineElementNotMergedAcrossPageSplit verifies that an inline
// element in the overflow keeps its own dedicated TextRun — not
// merged with surrounding text runs. The cloneWithWords sameRun guard
// must reject inline-vs-text and inline-vs-inline merges.
func TestInlineElementNotMergedAcrossPageSplit(t *testing.T) {
	el := &fixedElement{width: 20, height: 16}
	// Long prefix pushes "xx" + inline + "yy zz" into the overflow.
	prefix := strings.Repeat("alpha beta gamma delta ", 16)
	p := NewStyledParagraph(
		NewRun(prefix, font.Helvetica, 12),
		NewRun("xx ", font.Helvetica, 12),
		RunInline(el),
		NewRun(" yy zz", font.Helvetica, 12),
	)

	plan := p.PlanLayout(LayoutArea{Width: 80, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)

	inlineCount := 0
	for _, run := range overflow.Runs() {
		if run.InlineElement != nil {
			inlineCount++
			if run.Text != "" {
				t.Errorf("inline run also has text %q — must not be merged", run.Text)
			}
		}
	}
	if inlineCount == 0 {
		t.Fatal("test setup: inline element did not land in overflow")
	}
	if inlineCount != 1 {
		t.Errorf("expected exactly 1 inline run in overflow, got %d", inlineCount)
	}
}

// TestArabicOriginalTextSurvivesPageSplit verifies that an Arabic
// paragraph that overflows a page produces an overflow paragraph
// whose re-layout still populates Word.OriginalText. Before this fix,
// wordToRun used Word.Text (post-shape Presentation Forms-B) for the
// reconstructed TextRun.Text, so re-laying the overflow saw already-
// shaped text and skipped setting OriginalText, breaking /ActualText
// markers and copy/paste fidelity (ISO 32000-2 §14.9.4).
func TestArabicOriginalTextSurvivesPageSplit(t *testing.T) {
	arabic := "العربية"
	text := strings.Repeat(arabic+" ", 30)
	p := NewParagraph(text, font.Helvetica, 12)

	plan := p.PlanLayout(LayoutArea{Width: 100, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)

	lines := overflow.Layout(100)
	if len(lines) == 0 {
		t.Fatal("overflow has no lines")
	}
	hasOriginalText := false
	for _, line := range lines {
		for _, w := range line.Words {
			if w.OriginalText != "" && strings.Contains(w.OriginalText, arabic) {
				hasOriginalText = true
				break
			}
		}
		if hasOriginalText {
			break
		}
	}
	if !hasOriginalText {
		t.Error("Arabic OriginalText not recovered after page split — /ActualText markers will be missing")
	}
}

// TestInlineElementAtPageSplitBoundary covers the corner case where
// the inline element falls at the exact line boundary the splitter
// chose. The inline word becomes the first word of the overflow line
// and must still survive (no off-by-one in the head/tail allocation).
func TestInlineElementAtPageSplitBoundary(t *testing.T) {
	el := &fixedElement{width: 60, height: 16}
	p := NewStyledParagraph(
		NewRun(strings.Repeat("aa bb ", 4), font.Helvetica, 12),
		RunInline(el),
		NewRun(" cc dd ee ff", font.Helvetica, 12),
	)
	plan := p.PlanLayout(LayoutArea{Width: 70, Height: 18})
	if plan.Status != LayoutPartial {
		t.Skip("did not produce a partial plan; test setup needs tuning")
	}
	overflow := plan.Overflow.(*Paragraph)

	for _, run := range overflow.Runs() {
		if run.InlineElement == el {
			return
		}
	}
	t.Error("inline element at split boundary lost in overflow")
}

// TestStyledRunsAroundInlineElementSurvivePageSplit verifies that
// when text on either side of an inline element shares the same
// styling, the page-split reconstruction does NOT collapse the three
// runs into one or two — each region keeps its own TextRun. This
// exercises the !isInline && !curInline guards in cloneWithWords's
// sameRun check.
func TestStyledRunsAroundInlineElementSurvivePageSplit(t *testing.T) {
	el := &fixedElement{width: 20, height: 16}
	// Both text runs use identical styling so a naive sameRun check
	// would merge them. The inline must enforce a run boundary.
	style := func(s string) TextRun {
		return NewRun(s, font.Helvetica, 12).WithColor(ColorRed)
	}
	prefix := style(strings.Repeat("alpha beta gamma delta ", 16))
	p := NewStyledParagraph(
		prefix,
		style("before "),
		RunInline(el),
		style(" after"),
	)

	plan := p.PlanLayout(LayoutArea{Width: 80, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)

	// Find the index of the inline run; assert text exists both
	// before AND after it (i.e. three+ runs spanning the inline).
	runs := overflow.Runs()
	inlineIdx := -1
	for i, r := range runs {
		if r.InlineElement == el {
			inlineIdx = i
			break
		}
	}
	if inlineIdx == -1 {
		t.Fatal("inline element not in overflow; test setup needs tuning")
	}
	textBefore := false
	for _, r := range runs[:inlineIdx] {
		if r.Text != "" && r.InlineElement == nil {
			textBefore = true
			break
		}
	}
	textAfter := false
	for _, r := range runs[inlineIdx+1:] {
		if r.Text != "" && r.InlineElement == nil {
			textAfter = true
			break
		}
	}
	if !textBefore || !textAfter {
		t.Errorf("expected text runs on both sides of inline element in overflow; "+
			"textBefore=%v textAfter=%v runs=%d inlineIdx=%d",
			textBefore, textAfter, len(runs), inlineIdx)
	}
}

// TestInlineElementOnlyOverflowSurvivesPageSplit covers the corner
// case where the overflow region contains nothing but a single
// inline element (e.g. text fills page 1, the trailing image is the
// entire page-2 content). cloneWithWords initializes its run list
// from words[0]; if that word is inline, the loop skips and a single
// inline run must be emitted.
func TestInlineElementOnlyOverflowSurvivesPageSplit(t *testing.T) {
	el := &fixedElement{width: 60, height: 16}
	// Long text + trailing inline element, tight Height: text fills
	// page 1, inline is alone on page 2.
	prefix := strings.Repeat("alpha beta gamma delta ", 20)
	p := NewStyledParagraph(
		NewRun(prefix, font.Helvetica, 12),
		RunInline(el),
	)
	plan := p.PlanLayout(LayoutArea{Width: 80, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)

	for _, run := range overflow.Runs() {
		if run.InlineElement == el {
			return
		}
	}
	t.Errorf("inline element alone in overflow was lost; runs=%d", len(overflow.Runs()))
}

// TestDevanagariGIDsRegenerateAfterPageSplit verifies that an Indic
// (Devanagari) paragraph splitting across a page produces an overflow
// whose re-layout re-populates Word.GIDs. The shaper's GID output is
// not stored on TextRun (it has no GIDs field); instead, wordToRun
// puts the pre-shape OriginalText in TextRun.Text, and re-laying the
// cloned paragraph re-runs ShapeIndicWithEmbedded to regenerate GIDs.
func TestDevanagariGIDsRegenerateAfterPageSplit(t *testing.T) {
	face := newMockDevaFace()
	face.substitutions = &font.GSUBSubstitutions{
		Single: map[font.GSUBFeature]map[uint16]uint16{},
	}
	ef := font.NewEmbeddedFont(face)

	// "ka + i-matra" repeated; each word is a Devanagari syllable.
	syllable := "कि"
	text := strings.Repeat(syllable+" ", 30)
	p := NewStyledParagraph(NewRunEmbedded(text, ef, 12))

	plan := p.PlanLayout(LayoutArea{Width: 60, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)

	// Re-lay and check at least one word has a non-empty GID stream.
	lines := overflow.Layout(60)
	if len(lines) == 0 {
		t.Fatal("overflow has no lines")
	}
	hasGIDs := false
	for _, line := range lines {
		for _, w := range line.Words {
			if len(w.GIDs) > 0 {
				hasGIDs = true
				break
			}
		}
		if hasGIDs {
			break
		}
	}
	if !hasGIDs {
		t.Error("Devanagari GIDs not regenerated after page split — Identity-H text emission will fail")
	}
}
