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

// TestCJKNoSpuriousSpacesAfterPageSplit verifies that when a CJK
// paragraph crosses a page break, the reconstructed overflow text
// does not contain spurious ASCII spaces between ideographs.
//
// Before this fix, cloneWithWords joined same-style words with a
// literal " " regardless of the word's SpaceAfter. CJK ideographs
// have SpaceAfter=0 (no inter-word spacing); the join rule corrupted
// the output by inserting visible spaces, e.g. "中文" became "中 文"
// in any second-page continuation.
func TestCJKNoSpuriousSpacesAfterPageSplit(t *testing.T) {
	text := strings.Repeat("中文文本", 30)
	p := NewParagraph(text, font.Helvetica, 12)

	plan := p.PlanLayout(LayoutArea{Width: 60, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)

	// The reconstructed runs hold the joined text; no run should
	// contain an ASCII space between ideographs.
	for i, run := range overflow.Runs() {
		if strings.ContainsRune(run.Text, ' ') {
			t.Errorf("run %d text contains spurious space: %q", i, run.Text)
		}
	}
}

// TestCJKLineCountReconcilesAfterPageSplit verifies that the line
// count of head + tail matches the original. With the spurious-space
// bug, re-laying the overflow produced MORE lines than the original
// at the same width, because every inserted space became a real
// spaceW-wide gap during re-measurement.
func TestCJKLineCountReconcilesAfterPageSplit(t *testing.T) {
	text := strings.Repeat("中文文本", 30)
	p := NewParagraph(text, font.Helvetica, 12)
	totalLines := p.MeasureLines(60)
	if totalLines < 5 {
		t.Fatalf("test setup needs ≥5 wrapped lines; got %d (Helvetica CJK measurement may have changed)", totalLines)
	}

	plan := p.PlanLayout(LayoutArea{Width: 60, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)
	overflowLines := overflow.MeasureLines(60)

	consumedLines := totalLines - overflowLines
	if consumedLines < 1 {
		t.Errorf("overflow line count exceeds original (corruption likely): total=%d overflow=%d", totalLines, overflowLines)
	}
	if overflowLines == 0 {
		t.Errorf("overflow has zero lines despite LayoutPartial status")
	}
}

// TestMixedCJKLatinNoSpuriousSpacesAfterPageSplit verifies that when
// CJK and Latin text mix in the same paragraph, page-split joins:
//   - preserve glued boundaries (CJK adjacent to Latin without source space)
//   - preserve real source spaces (one and only one between Latin words)
//   - never insert spaces between adjacent CJK characters
func TestMixedCJKLatinNoSpuriousSpacesAfterPageSplit(t *testing.T) {
	// Source has explicit spaces only between "word" and the next CJK chunk.
	text := strings.Repeat("中文word ", 30)
	p := NewParagraph(text, font.Helvetica, 12)

	plan := p.PlanLayout(LayoutArea{Width: 80, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)

	for i, run := range overflow.Runs() {
		// "中 文" or "文 word" or "word中" would all indicate corruption.
		// The intended forms are "中文word" (glued) and "word 中" (source space).
		if strings.Contains(run.Text, "中 ") || strings.Contains(run.Text, "文 中") || strings.Contains(run.Text, "文 文") {
			t.Errorf("run %d: spurious space between CJK chars: %q", i, run.Text)
		}
		// "word中" without an intervening space would indicate the source
		// space was dropped. The source had "word " before each "中".
		if strings.Contains(run.Text, "word中") {
			t.Errorf("run %d: source space dropped between Latin and CJK: %q", i, run.Text)
		}
	}
}

// TestNoHyphenInPageSplitOverflow documents and locks in the assumption
// that hyphenation does NOT run on the PlanLayout path. hyphenateWord
// is invoked only inside Paragraph.Layout (paragraph.go line 390), not
// in wrapWords (the helper PlanLayout uses). So the overflow paragraph
// reconstructed by cloneWithWords never sees the hyphen-with-SpaceAfter=0
// word pair that would otherwise need a hyphen-aware join fix.
//
// If this test ever fails, the hyphen-aware join logic in cloneWithWords
// becomes a real bug, not a latent one — see the breakLongWords follow-up
// issue and the wordToRun doc-comment.
func TestNoHyphenInPageSplitOverflow(t *testing.T) {
	// A paragraph with hyphens: auto and a long alpha word that would
	// hyphenate under both the Liang-Knuth pattern path and the
	// character-boundary fallback in hyphenateWord. Width 60pt at
	// Helvetica 12pt is tight enough that no whole copy of the word
	// fits on a line — so hyphenation would be the only way to break
	// it if wrapWords ever called hyphenateWord. Source contains no "-",
	// so any hyphen in the overflow text must have come from the
	// hyphenator.
	long := strings.Repeat("antidisestablishmentarianism ", 8)
	p := NewParagraph(long, font.Helvetica, 12).SetHyphens("auto")

	plan := p.PlanLayout(LayoutArea{Width: 60, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)
	if len(overflow.Runs()) == 0 {
		t.Fatalf("overflow has zero runs — assertion below would pass vacuously")
	}

	for i, run := range overflow.Runs() {
		if strings.Contains(run.Text, "-") {
			t.Errorf("run %d contains hyphen in overflow — wrapWords path now hyphenates, "+
				"cloneWithWords needs hyphen-aware join: %q", i, run.Text)
		}
	}
}

// TestLineBreakResetsJoinStateAcrossPageSplit verifies that a forced
// linebreak (\n) in the middle of a CJK paragraph splits cleanly
// across a page boundary — the LineBreak branch must update
// prevSpaceAfter so the post-break word joins correctly.
func TestLineBreakResetsJoinStateAcrossPageSplit(t *testing.T) {
	// Several lines of CJK with explicit \n breaks; long enough to
	// force a page split somewhere mid-paragraph.
	chunk := "中文文本"
	text := ""
	for i := 0; i < 20; i++ {
		if i > 0 {
			text += "\n"
		}
		text += chunk
	}
	p := NewParagraph(text, font.Helvetica, 12)

	plan := p.PlanLayout(LayoutArea{Width: 60, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)

	for i, run := range overflow.Runs() {
		if strings.Contains(run.Text, " ") {
			t.Errorf("run %d contains spurious space after linebreak split: %q", i, run.Text)
		}
	}
}
