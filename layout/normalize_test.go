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
	precomposedCafe = "caf\u00e9"
	decomposedCafe  = "cafe\u0301"
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
// every run it receives, not just the first one.
func TestNewStyledParagraphNFC(t *testing.T) {
	r1 := TextRun{Text: decomposedCafe, Font: font.Helvetica, FontSize: 12}
	r2 := TextRun{Text: "e\u0301", Font: font.Helvetica, FontSize: 12}
	p := NewStyledParagraph(r1, r2)

	if p.runs[0].Text != precomposedCafe {
		t.Errorf("run 0: got %q, want %q", p.runs[0].Text, precomposedCafe)
	}
	if p.runs[1].Text != "\u00e9" {
		t.Errorf("run 1: got %q, want %q", p.runs[1].Text, "\u00e9")
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
// text in each incoming run.
func TestHeadingSetRunsNFC(t *testing.T) {
	h := NewHeading("", H1)
	r := TextRun{Text: decomposedCafe, Font: font.HelveticaBold, FontSize: 28}
	h.SetRuns([]TextRun{r})
	if h.para.runs[0].Text != precomposedCafe {
		t.Errorf("heading run text: got %q, want %q",
			h.para.runs[0].Text, precomposedCafe)
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
