// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
)

// Decomposed "café" = c + a + f + e + U+0301 (combining acute accent).
// Precomposed "café" = c + a + f + U+00E9.
const (
	precomposedCafe   = "caf\u00e9"
	decomposedCafe    = "cafe\u0301"
	precomposedEAcute = "\u00e9"
	decomposedEAcute  = "e\u0301"
)

// TestNormalizeTextNFC covers the normalizeText helper directly: NFC
// maps decomposed input to precomposed, leaves already-composed input
// byte-identical, and is idempotent on repeated application.
func TestNormalizeTextNFC(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"ascii", "Hello", "Hello"},
		{"already_nfc", precomposedCafe, precomposedCafe},
		{"decomposed_to_nfc", decomposedCafe, precomposedCafe},
		{"mixed", "na\u00efve cafe\u0301", "na\u00efve caf\u00e9"},
		// Combining diaeresis over i: i + U+0308 -> U+00EF.
		{"combining_diaeresis", "na\u0069\u0308ve", "na\u00efve"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeText(tc.in)
			if got != tc.want {
				t.Errorf("normalizeText(%q) = %q, want %q", tc.in, got, tc.want)
			}
			// Idempotence: normalizing again must not change the result.
			if got2 := normalizeText(got); got2 != got {
				t.Errorf("normalizeText not idempotent: %q -> %q", got, got2)
			}
		})
	}
}

// TestNormalizeTextPassthroughWhenAlreadyNFC verifies the fast-path
// optimization does not rewrite already-NFC input.
func TestNormalizeTextPassthroughWhenAlreadyNFC(t *testing.T) {
	in := "ASCII-only string with no combining marks"
	out := normalizeText(in)
	if out != in {
		t.Errorf("already-NFC input changed: got %q", out)
	}
}

// TestParagraphNFCAtConstructor verifies NewParagraph normalizes user
// input so a decomposed string becomes byte-identical to its precomposed
// equivalent before any layout work happens.
func TestParagraphNFCAtConstructor(t *testing.T) {
	pComposed := NewParagraph(precomposedCafe, font.Helvetica, 12)
	pDecomposed := NewParagraph(decomposedCafe, font.Helvetica, 12)

	if pComposed.runs[0].Text != precomposedCafe {
		t.Errorf("composed run: got %q, want %q",
			pComposed.runs[0].Text, precomposedCafe)
	}
	if pDecomposed.runs[0].Text != precomposedCafe {
		t.Errorf("decomposed run: got %q, want %q (NFC normalization expected)",
			pDecomposed.runs[0].Text, precomposedCafe)
	}
}

// TestNewStyledParagraphNFC verifies that NewStyledParagraph normalizes
// every run it receives, not just the first one. It also verifies that
// the caller's input slice is left untouched: NewStyledParagraph must
// allocate a fresh slice for the paragraph's internal storage so the
// caller can safely re-use the slice afterward.
func TestNewStyledParagraphNFC(t *testing.T) {
	r1 := TextRun{Text: decomposedCafe, Font: font.Helvetica, FontSize: 12}
	r2 := TextRun{Text: decomposedEAcute, Font: font.Helvetica, FontSize: 12}
	input := []TextRun{r1, r2}
	p := NewStyledParagraph(input...)

	if p.runs[0].Text != precomposedCafe {
		t.Errorf("run 0: got %q, want %q", p.runs[0].Text, precomposedCafe)
	}
	if p.runs[1].Text != precomposedEAcute {
		t.Errorf("run 1: got %q, want %q", p.runs[1].Text, precomposedEAcute)
	}
	if input[0].Text != decomposedCafe {
		t.Errorf("caller slice mutated at index 0: got %q, want %q",
			input[0].Text, decomposedCafe)
	}
	if input[1].Text != decomposedEAcute {
		t.Errorf("caller slice mutated at index 1: got %q, want %q",
			input[1].Text, decomposedEAcute)
	}
}

// TestAddRunNFC verifies that Paragraph.AddRun normalizes the run's
// text before storing it.
func TestAddRunNFC(t *testing.T) {
	p := NewParagraph("", font.Helvetica, 12)
	p.AddRun(TextRun{Text: decomposedCafe, Font: font.Helvetica, FontSize: 12})
	if p.runs[1].Text != precomposedCafe {
		t.Errorf("AddRun text: got %q, want %q", p.runs[1].Text, precomposedCafe)
	}
}

// TestNewRunNFC verifies the TextRun constructors normalize input.
func TestNewRunNFC(t *testing.T) {
	r := NewRun(decomposedCafe, font.Helvetica, 12)
	if r.Text != precomposedCafe {
		t.Errorf("NewRun: got %q, want %q", r.Text, precomposedCafe)
	}
	re := NewRunEmbedded(decomposedCafe, nil, 12)
	if re.Text != precomposedCafe {
		t.Errorf("NewRunEmbedded: got %q, want %q", re.Text, precomposedCafe)
	}
}

// TestParagraphNFCByteIdenticalLayout drives the full layout pass and
// verifies that decomposed and precomposed inputs produce identical
// Word streams. Same Text on the shaped word, same Width. This is the
// invariant that protects shaping and measurement from ever seeing the
// decomposed form.
func TestParagraphNFCByteIdenticalLayout(t *testing.T) {
	pComposed := NewParagraph(precomposedCafe, font.Helvetica, 12)
	pDecomposed := NewParagraph(decomposedCafe, font.Helvetica, 12)

	linesComposed := pComposed.Layout(500)
	linesDecomposed := pDecomposed.Layout(500)

	if len(linesComposed) != len(linesDecomposed) {
		t.Fatalf("line counts differ: composed=%d, decomposed=%d",
			len(linesComposed), len(linesDecomposed))
	}
	for i, lc := range linesComposed {
		ld := linesDecomposed[i]
		if len(lc.Words) != len(ld.Words) {
			t.Errorf("line %d: word counts differ: composed=%d, decomposed=%d",
				i, len(lc.Words), len(ld.Words))
			continue
		}
		for j, wc := range lc.Words {
			wd := ld.Words[j]
			if wc.Text != wd.Text {
				t.Errorf("line %d word %d: text differs: composed=%q, decomposed=%q",
					i, j, wc.Text, wd.Text)
			}
			if wc.Width != wd.Width {
				t.Errorf("line %d word %d: width differs: composed=%v, decomposed=%v",
					i, j, wc.Width, wd.Width)
			}
			if wc.OriginalText != wd.OriginalText {
				t.Errorf("line %d word %d: OriginalText differs: composed=%q, decomposed=%q",
					i, j, wc.OriginalText, wd.OriginalText)
			}
		}
	}
}

// TestParagraphNFCByteIdenticalDrawOutput renders both paragraphs
// through drawTextLine and verifies the resulting content streams are
// byte-identical.
func TestParagraphNFCByteIdenticalDrawOutput(t *testing.T) {
	renderFirstLine := func(text string) []byte {
		p := NewParagraph(text, font.Helvetica, 12)
		lines := p.Layout(500)
		if len(lines) == 0 {
			return nil
		}
		stream := content.NewStream()
		page := &PageResult{Stream: stream}
		ctx := DrawContext{
			Stream:     stream,
			Page:       page,
			ActualText: true,
		}
		line := lines[0]
		drawTextLine(ctx, line.Words, 0, 100, 500, line.Align, line.IsLast)
		return stream.Bytes()
	}
	composed := renderFirstLine(precomposedCafe)
	decomposed := renderFirstLine(decomposedCafe)
	if len(composed) == 0 {
		t.Fatalf("composed render produced empty stream")
	}
	if len(decomposed) == 0 {
		t.Fatalf("decomposed render produced empty stream")
	}
	if !bytes.Equal(composed, decomposed) {
		t.Errorf("content streams differ:\ncomposed: %q\ndecomposed: %q",
			string(composed), string(decomposed))
	}
}

// TestParagraphAlreadyNFCPassthrough verifies already-NFC input flows
// through the constructors unchanged at the byte level. This guards
// the fast path and ensures we never disturb ASCII or pre-composed
// text.
func TestParagraphAlreadyNFCPassthrough(t *testing.T) {
	in := "The quick brown fox jumps over the lazy dog."
	p := NewParagraph(in, font.Helvetica, 12)
	if p.runs[0].Text != in {
		t.Errorf("already-NFC ASCII changed: got %q, want %q", p.runs[0].Text, in)
	}
	mixed := "Crème brûlée, " + precomposedCafe
	p2 := NewParagraph(mixed, font.Helvetica, 12)
	if p2.runs[0].Text != mixed {
		t.Errorf("already-NFC mixed changed: got %q, want %q", p2.runs[0].Text, mixed)
	}
}

// TestParagraphMixedPartialNormalization covers the case where only
// part of the input string is decomposed. The entire string should
// come out in NFC.
func TestParagraphMixedPartialNormalization(t *testing.T) {
	in := "Hello cafe\u0301 world"
	want := "Hello caf\u00e9 world"
	p := NewParagraph(in, font.Helvetica, 12)
	if p.runs[0].Text != want {
		t.Errorf("mixed text: got %q, want %q", p.runs[0].Text, want)
	}
}

// TestArabicActualTextStillEmitted guards against a regression where
// NFC normalization (which happens before shaping) might interfere
// with the existing ActualText fallback used by the Arabic shaper.
// Arabic codepoints in this test are already NFC, so normalization
// is a no-op; shaping should still populate Word.OriginalText.
func TestArabicActualTextStillEmitted(t *testing.T) {
	const arabic = "\u0645\u0631\u062d\u0628\u0627" // marhaba (hello)
	p := NewParagraph(arabic, font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatalf("expected at least one word")
	}
	w := lines[0].Words[0]
	if w.OriginalText == "" {
		t.Errorf("expected OriginalText to be set after Arabic shaping")
	}
	if w.OriginalText != arabic {
		t.Errorf("OriginalText should hold the pre-shaping codepoints: got %q, want %q",
			w.OriginalText, arabic)
	}
}

// TestArabicDecomposedNormalizedBeforeShaping exercises the full Arabic
// path with input that NFC actually has work to do on. Alef (U+0627)
// followed by combining hamza-above (U+0654) decomposes to the same
// canonical equivalent as alef-with-hamza-above (U+0623). The Arabic
// shaper expects the precomposed form; without NFC it would see two
// codepoints and the OriginalText recovery would carry the decomposed
// sequence, breaking copy/paste and accessibility.
func TestArabicDecomposedNormalizedBeforeShaping(t *testing.T) {
	const decomposed = "\u0627\u0654" // alef + combining hamza-above
	const precomposed = "\u0623"      // alef with hamza-above
	p := NewParagraph(decomposed, font.Helvetica, 12)
	if got := p.runs[0].Text; got != precomposed {
		t.Fatalf("paragraph run text: got %q, want %q", got, precomposed)
	}
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatalf("expected at least one word")
	}
	w := lines[0].Words[0]
	// OriginalText is set to the pre-shape text only when the shaped
	// Text differs from the input. For a single precomposed alef the
	// Arabic shaper produces a positional form, so OriginalText holds
	// the precomposed (post-NFC) input.
	if w.OriginalText != "" && w.OriginalText != precomposed {
		t.Errorf("OriginalText after NFC: got %q, want precomposed %q",
			w.OriginalText, precomposed)
	}
}

// TestTableCellNFC verifies AddCell normalizes its text input so
// downstream measurement sees the NFC form.
func TestTableCellNFC(t *testing.T) {
	tbl := NewTable()
	row := tbl.AddRow()
	row.AddCell(decomposedCafe, font.Helvetica, 12)
	if row.cells[0].text != precomposedCafe {
		t.Errorf("cell text: got %q, want %q", row.cells[0].text, precomposedCafe)
	}
}

// TestHeadingSetRunsNFC verifies that Heading.SetRuns normalizes the
// text in each incoming run and does not mutate the caller's slice.
func TestHeadingSetRunsNFC(t *testing.T) {
	h := NewHeading("", H1)
	r := TextRun{Text: decomposedCafe, Font: font.HelveticaBold, FontSize: 28}
	input := []TextRun{r}
	h.SetRuns(input)
	if h.para.runs[0].Text != precomposedCafe {
		t.Errorf("heading run text: got %q, want %q",
			h.para.runs[0].Text, precomposedCafe)
	}
	if input[0].Text != decomposedCafe {
		t.Errorf("caller slice mutated: got %q, want %q",
			input[0].Text, decomposedCafe)
	}
}

// TestHeadingSetBookmarkLabelNFC verifies that SetBookmarkLabel
// normalizes the label text. Bookmark labels feed PDF metadata, not
// flowed text, but normalization keeps the layout-boundary API
// uniform so callers do not have to think about which entry points
// canonicalize and which do not.
func TestHeadingSetBookmarkLabelNFC(t *testing.T) {
	h := NewHeading("Title", H1)
	h.SetBookmarkLabel(decomposedCafe)
	if h.bookmarkLabel != precomposedCafe {
		t.Errorf("bookmark label: got %q, want %q",
			h.bookmarkLabel, precomposedCafe)
	}
}

// TestTabbedLineSetSegmentsNFC verifies that TabbedLine.SetSegments
// normalizes every segment it stores.
func TestTabbedLineSetSegmentsNFC(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12, TabStop{Position: 100})
	tl.SetSegments(precomposedCafe, decomposedCafe)
	if tl.segments[0] != precomposedCafe {
		t.Errorf("segment 0: got %q, want %q", tl.segments[0], precomposedCafe)
	}
	if tl.segments[1] != precomposedCafe {
		t.Errorf("segment 1 (decomposed): got %q, want %q",
			tl.segments[1], precomposedCafe)
	}
}

// TestNFCTestStringsIntegrity guards against editor or tool-level
// normalization silently rewriting the string literals used in this
// file.
func TestNFCTestStringsIntegrity(t *testing.T) {
	if strings.Contains(precomposedCafe, "\u0301") {
		t.Errorf("precomposedCafe unexpectedly contains combining acute")
	}
	if !strings.Contains(decomposedCafe, "\u0301") {
		t.Errorf("decomposedCafe missing combining acute")
	}
}

// TestNormalizeTextVietnamese3Stack verifies multi-mark stacks compose.
// Latin "a" + combining circumflex (U+0302) + combining grave (U+0300)
// is the canonical decomposition of "a-with-circumflex-and-grave"
// (U+1EA7), the Vietnamese "ầ". Three-mark stacks like this are the
// case where naive cmap-only fonts fall over without NFC.
func TestNormalizeTextVietnamese3Stack(t *testing.T) {
	in := "ca\u0302\u0300"
	want := "c\u1ea7"
	if got := normalizeText(in); got != want {
		t.Errorf("Vietnamese 3-stack: got %q, want %q", got, want)
	}
}

// TestNormalizeTextHangulJamo verifies that L+V+T jamo collapse into a
// single precomposed Hangul syllable. U+1100 (L choseong kiyeok) +
// U+1161 (V jungseong a) + U+11A8 (T jongseong kiyeok) compose to
// U+AC01 (HANGUL SYLLABLE GAG). NFC handles this via the Hangul
// composition algorithm in UAX #15 §16, separately from the general
// canonical composition table.
func TestNormalizeTextHangulJamo(t *testing.T) {
	in := "\u1100\u1161\u11a8"
	want := "\uac01"
	if got := normalizeText(in); got != want {
		t.Errorf("Hangul jamo: got %q, want %q", got, want)
	}
}

// TestNormalizeTextSupplementaryPlane verifies that NFC does not corrupt
// codepoints above U+FFFF. U+1D49C (MATHEMATICAL SCRIPT CAPITAL A) is a
// 4-byte UTF-8 sequence that requires a surrogate pair when re-encoded
// to UTF-16 inside the normalizer; the helper must round-trip it
// byte-identically.
func TestNormalizeTextSupplementaryPlane(t *testing.T) {
	in := "\U0001d49c" // MATHEMATICAL SCRIPT CAPITAL A
	got := normalizeText(in)
	if got != in {
		t.Errorf("supplementary-plane char altered: got %q, want %q", got, in)
	}
	if len(got) != 4 {
		t.Errorf("UTF-8 length corrupted: got %d bytes, want 4", len(got))
	}
}

// TestNormalizeTextInvalidUTF8 pins the contract for ill-formed UTF-8.
// norm.NFC passes invalid byte sequences through unchanged; this test
// asserts that contract so a future swap to a different normalizer (or
// a stricter validation step) is forced to make the change explicit.
// The contract: never panic; ill-formed bytes survive untouched.
func TestNormalizeTextInvalidUTF8(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("normalizeText panicked on invalid UTF-8: %v", r)
		}
	}()
	in := "\xff\xfe abc"
	got := normalizeText(in)
	if got != in {
		t.Errorf("invalid UTF-8 not preserved: got %q, want %q", got, in)
	}
}

// TestNormalizeTextMixedScripts verifies that a string mixing
// decomposed Latin, Arabic, and Devanagari survives NFC with its
// script-segmentation boundaries intact. Decomposed Latin ("e\u0301")
// composes; Arabic and Devanagari runes are already in NFC and pass
// through; the script transitions remain detectable to the bidi /
// script splitter (splitMixedBidiWord) downstream.
func TestNormalizeTextMixedScripts(t *testing.T) {
	// Decomposed café + " " + Arabic "marhaba" + " " + Devanagari "नमस्ते"
	in := decomposedCafe + " \u0645\u0631\u062d\u0628\u0627 \u0928\u092e\u0938\u094d\u0924\u0947"
	want := precomposedCafe + " \u0645\u0631\u062d\u0628\u0627 \u0928\u092e\u0938\u094d\u0924\u0947"
	got := normalizeText(in)
	if got != want {
		t.Errorf("mixed-script NFC: got %q, want %q", got, want)
	}
	// Spot-check the script boundaries: Latin block, then Arabic, then
	// Devanagari should still be present in their respective Unicode
	// ranges so the script splitter can find the transitions.
	hasLatin, hasArabic, hasDevanagari := false, false, false
	for _, r := range got {
		switch {
		case r >= 'a' && r <= 'z':
			hasLatin = true
		case r >= 0x0600 && r <= 0x06ff:
			hasArabic = true
		case r >= 0x0900 && r <= 0x097f:
			hasDevanagari = true
		}
	}
	if !hasLatin || !hasArabic || !hasDevanagari {
		t.Errorf("script segmentation lost: latin=%v arabic=%v devanagari=%v",
			hasLatin, hasArabic, hasDevanagari)
	}
}

// TestListAddItemNFC guards against a future refactor that bypasses
// itemParagraph in List.Layout. AddItem stores the item text directly
// on listItem.text; List.MinWidth and List.MaxWidth read it through
// itemText, which feeds font.TextMeasurer with no further
// normalization. Without NFC at AddItem, those measurements would see
// decomposed input.
func TestListAddItemNFC(t *testing.T) {
	l := NewList(font.Helvetica, 12).AddItem(decomposedCafe)
	if l.items[0].text != precomposedCafe {
		t.Errorf("list item text: got %q, want %q",
			l.items[0].text, precomposedCafe)
	}
	// Layout path: the paragraph constructed for the item must also
	// emit a single Word whose Text equals the precomposed form.
	lines := l.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatalf("expected at least one word from list layout")
	}
	if lines[0].Words[0].Text != precomposedCafe {
		t.Errorf("list layout word: got %q, want %q",
			lines[0].Words[0].Text, precomposedCafe)
	}
}

// TestListAddItemRunsNFC verifies that AddItemRuns normalizes every run
// it stores and does not mutate the caller's slice. This protects List
// measurement (MinWidth/MaxWidth) against decomposed input that would
// otherwise reach font.TextMeasurer through the itemText concatenation
// path.
func TestListAddItemRunsNFC(t *testing.T) {
	r := TextRun{Text: decomposedCafe, Font: font.Helvetica, FontSize: 12}
	input := []TextRun{r}
	l := NewList(font.Helvetica, 12).AddItemRuns(input)
	if l.items[0].runs[0].Text != precomposedCafe {
		t.Errorf("list run text: got %q, want %q",
			l.items[0].runs[0].Text, precomposedCafe)
	}
	if input[0].Text != decomposedCafe {
		t.Errorf("caller slice mutated: got %q, want %q",
			input[0].Text, decomposedCafe)
	}
}

// TestListMaxWidthSeesNFC is the regression test for the original
// reviewer concern: List.MaxWidth measures the concatenated item text
// before Layout runs. Decomposed input must produce the same
// measurement as precomposed input; otherwise width planning is
// wrong for tables, page breaks, and overflow.
func TestListMaxWidthSeesNFC(t *testing.T) {
	composed := NewList(font.Helvetica, 12).AddItem(precomposedCafe)
	decomposed := NewList(font.Helvetica, 12).AddItem(decomposedCafe)
	if composed.MaxWidth() != decomposed.MaxWidth() {
		t.Errorf("MaxWidth differs: composed=%v, decomposed=%v",
			composed.MaxWidth(), decomposed.MaxWidth())
	}
	if composed.MinWidth() != decomposed.MinWidth() {
		t.Errorf("MinWidth differs: composed=%v, decomposed=%v",
			composed.MinWidth(), decomposed.MinWidth())
	}
}

// TestParagraphEmbeddedNFCByteIdenticalLayout exercises the embedded
// font path. The mock Arabic embedded font from kashida_paragraph_test.go
// gives deterministic widths, so this test runs without a system font
// fixture. It feeds two canonically equivalent inputs (alef + combining
// hamza-above vs. alef-with-hamza-above) into NewParagraphEmbedded and
// asserts the resulting Word stream is byte-identical: same Text, same
// Width, same OriginalText.
func TestParagraphEmbeddedNFCByteIdenticalLayout(t *testing.T) {
	ef := newMockArabicEmbedded()
	const decomposed = "\u0627\u0654" // alef + combining hamza-above
	const precomposed = "\u0623"      // alef with hamza-above

	pComposed := NewParagraphEmbedded(precomposed, ef, 12)
	pDecomposed := NewParagraphEmbedded(decomposed, ef, 12)

	if pComposed.runs[0].Text != precomposed {
		t.Fatalf("composed run text: got %q, want %q",
			pComposed.runs[0].Text, precomposed)
	}
	if pDecomposed.runs[0].Text != precomposed {
		t.Fatalf("decomposed run text: got %q, want %q (NFC expected)",
			pDecomposed.runs[0].Text, precomposed)
	}

	linesC := pComposed.Layout(500)
	linesD := pDecomposed.Layout(500)
	if len(linesC) != len(linesD) {
		t.Fatalf("line counts differ: composed=%d, decomposed=%d",
			len(linesC), len(linesD))
	}
	for i, lc := range linesC {
		ld := linesD[i]
		if len(lc.Words) != len(ld.Words) {
			t.Fatalf("line %d word counts differ: composed=%d, decomposed=%d",
				i, len(lc.Words), len(ld.Words))
		}
		for j, wc := range lc.Words {
			wd := ld.Words[j]
			if wc.Text != wd.Text {
				t.Errorf("line %d word %d Text: composed=%q, decomposed=%q",
					i, j, wc.Text, wd.Text)
			}
			if wc.Width != wd.Width {
				t.Errorf("line %d word %d Width: composed=%v, decomposed=%v",
					i, j, wc.Width, wd.Width)
			}
			if wc.OriginalText != wd.OriginalText {
				t.Errorf("line %d word %d OriginalText: composed=%q, decomposed=%q",
					i, j, wc.OriginalText, wd.OriginalText)
			}
		}
	}
}

// TestParagraphEmbeddedNFCByteIdenticalDrawOutput renders both
// canonically-equivalent inputs through drawTextLine on the mock
// Arabic embedded font and asserts the resulting content streams are
// byte-identical. Together with the layout-level test above, this
// closes the "embedded font + Arabic shaping" gap left by the existing
// Helvetica byte-identical tests.
func TestParagraphEmbeddedNFCByteIdenticalDrawOutput(t *testing.T) {
	ef := newMockArabicEmbedded()
	render := func(text string) []byte {
		p := NewParagraphEmbedded(text, ef, 12)
		lines := p.Layout(500)
		if len(lines) == 0 {
			return nil
		}
		stream := content.NewStream()
		page := &PageResult{Stream: stream}
		ctx := DrawContext{Stream: stream, Page: page, ActualText: true}
		line := lines[0]
		drawTextLine(ctx, line.Words, 0, 100, 500, line.Align, line.IsLast)
		return stream.Bytes()
	}
	composed := render("\u0622")          // alef with madda above (precomposed)
	decomposed := render("\u0627\u0653") // alef + combining madda above (decomposed)
	if len(composed) == 0 {
		t.Fatalf("composed render produced empty stream")
	}
	if len(decomposed) == 0 {
		t.Fatalf("decomposed render produced empty stream")
	}
	if !bytes.Equal(composed, decomposed) {
		t.Errorf("embedded-font streams differ:\ncomposed: %q\ndecomposed: %q",
			string(composed), string(decomposed))
	}
}
