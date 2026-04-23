// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
)

// mockGPOSFace is a deterministic Face + GPOSProvider used to exercise
// drawWordEmbeddedWithMarks. Each rune is mapped to a GID by a lookup
// table; advances and upem are fixed; GPOS data is injected directly.
type mockGPOSFace struct {
	upem    int
	advance map[uint16]int
	cmap    map[rune]uint16
	gpos    *font.GPOSAdjustments
}

func (m *mockGPOSFace) PostScriptName() string { return "MockGPOSFace" }
func (m *mockGPOSFace) UnitsPerEm() int        { return m.upem }
func (m *mockGPOSFace) GlyphIndex(r rune) uint16 {
	return m.cmap[r]
}
func (m *mockGPOSFace) GlyphAdvance(gid uint16) int {
	return m.advance[gid]
}
func (m *mockGPOSFace) Ascent() int             { return 800 }
func (m *mockGPOSFace) Descent() int            { return -200 }
func (m *mockGPOSFace) BBox() [4]int            { return [4]int{0, -200, 1000, 800} }
func (m *mockGPOSFace) ItalicAngle() float64    { return 0 }
func (m *mockGPOSFace) CapHeight() int          { return 700 }
func (m *mockGPOSFace) StemV() int              { return 80 }
func (m *mockGPOSFace) Kern(uint16, uint16) int { return 0 }
func (m *mockGPOSFace) Flags() uint32           { return 0 }
func (m *mockGPOSFace) RawData() []byte         { return nil }
func (m *mockGPOSFace) NumGlyphs() int          { return 4096 }

// GPOS satisfies font.GPOSProvider.
func (m *mockGPOSFace) GPOS() *font.GPOSAdjustments { return m.gpos }

// newLamFathaFace constructs a mock face with lam (U+0644) as a base
// glyph and fatha (U+064E) as a combining mark, plus a single GPOS
// mark/base entry that attaches fatha on class 0 of lam.
// Anchors: base lam at (500, 800), mark fatha at (200, 300).
// Expected MarkOffset = (500-200, 800-300) = (300, 500).
func newLamFathaFace() *mockGPOSFace {
	const (
		lamGID   uint16 = 50
		fathaGID uint16 = 60
	)
	face := &mockGPOSFace{
		upem: 1000,
		advance: map[uint16]int{
			lamGID:   700,
			fathaGID: 0, // combining mark: zero advance
		},
		cmap: map[rune]uint16{
			0x0644: lamGID,
			0x064E: fathaGID,
		},
		gpos: &font.GPOSAdjustments{
			Pairs: map[font.GPOSFeature]map[[2]uint16]font.PairAdjustment{},
			Marks: map[font.GPOSFeature]map[uint16]font.MarkRecord{
				font.GPOSMark: {
					fathaGID: {Class: 0, Anchor: font.Anchor{X: 200, Y: 300}},
				},
			},
			Bases: map[font.GPOSFeature]map[uint16]font.BaseRecord{
				font.GPOSMark: {
					lamGID: {Anchors: []font.Anchor{{X: 500, Y: 800}}},
				},
			},
		},
	}
	return face
}

// capturedWordStream renders a single Word in isolation through
// drawWordEmbedded bracketed by BT/ET/MoveText and returns the
// resulting raw content-stream bytes. Mirrors the operator sequence
// that drawTextLine would produce for this word.
func capturedWordStream(word Word) []byte {
	s := content.NewStream()
	s.BeginText()
	s.SetFont("F1", word.FontSize)
	s.MoveText(0, 0)
	drawWordEmbedded(s, word)
	s.EndText()
	return s.Bytes()
}

// countTdOps counts Td operator occurrences in a content stream.
func countTdOps(b []byte) int {
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " Td") {
			n++
		}
	}
	return n
}

// firstMarkTdBetween returns the first Td operator line that appears
// strictly between two Tj hex-string lines in the given stream. It is
// used to pick out the Td that drawWordEmbeddedWithMarks inserts
// between the base Tj and the mark Tj. Returns the empty string when
// no such Td exists.
func firstMarkTdBetween(b []byte) string {
	lines := strings.Split(string(b), "\n")
	seenTj := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if seenTj && strings.HasSuffix(line, " Td") {
			return line
		}
		if strings.HasSuffix(line, " Tj") {
			if seenTj {
				return "" // two Tjs with no Td between them
			}
			seenTj = true
		}
	}
	return ""
}

// TestGPOSMarkAttachmentArabicHaraka renders a lam+fatha cluster with
// a mock face that carries a GPOS mark-to-base entry and asserts the
// content stream contains a Td move before the fatha Tj whose operands
// match the expected offset, plus a matching Td move back after.
func TestGPOSMarkAttachmentArabicHaraka(t *testing.T) {
	face := newLamFathaFace()
	ef := font.NewEmbeddedFont(face)

	// Expected offsets in points at fontSize=12:
	//   dx = (500 - 200) / 1000 * 12 = 3.6
	//   dy = (800 - 300) / 1000 * 12 = 6.0
	//   baseAdvance = 700 / 1000 * 12 = 8.4
	//   First Td: (dx - baseAdvance, dy) = (-4.8, 6)
	//   Second Td: (baseAdvance - dx, -dy) = (4.8, -6)
	word := Word{
		Text:     "\u0644\u064E", // lam + fatha
		Embedded: ef,
		FontSize: 12,
	}

	b := capturedWordStream(word)
	if countTdOps(b) < 3 {
		// One Td for the initial MoveText, two for the mark bracket.
		t.Fatalf("expected at least 3 Td ops (initial + mark bracket), got %d:\n%s", countTdOps(b), b)
	}

	// Confirm the first Td between Tj lines is the mark-open shift.
	td := firstMarkTdBetween(b)
	if td == "" {
		t.Fatalf("no Td between base Tj and mark Tj:\n%s", b)
	}
	if !strings.Contains(td, "-4.8") || !strings.Contains(td, "6 Td") {
		t.Errorf("mark-open Td operands: got %q, want -4.8 and 6:\n%s", td, b)
	}

	// Confirm the closing +4.8 / -6 Td appears somewhere after it.
	if !bytes.Contains(b, []byte("4.8 -6 Td")) {
		t.Errorf("expected closing Td '4.8 -6 Td' in stream:\n%s", b)
	}
}

// TestGPOSMarkAttachmentNoGPOSFallback verifies that when the font has
// no GPOS mark data, drawWordEmbedded emits the cluster via the fast
// path (single Tj, no Td pairs between glyph emissions). The only Td
// remains the initial MoveText.
func TestGPOSMarkAttachmentNoGPOSFallback(t *testing.T) {
	face := newLamFathaFace()
	face.gpos = nil
	ef := font.NewEmbeddedFont(face)

	word := Word{
		Text:     "\u0644\u064E",
		Embedded: ef,
		FontSize: 12,
	}

	b := capturedWordStream(word)
	// Exactly one Td: the initial MoveText(0, 0).
	if countTdOps(b) != 1 {
		t.Errorf("expected exactly 1 Td (initial MoveText), got %d:\n%s", countTdOps(b), b)
	}
	if firstMarkTdBetween(b) != "" {
		t.Errorf("unexpected Td between Tj lines without GPOS:\n%s", b)
	}
}

// TestGPOSMarkAttachmentLatinUntouched asserts Latin-only words that
// contain no combining marks are emitted by the fast path: no Td moves
// between Tjs, and the output is byte-for-byte what the pre-GPOS path
// would have produced.
func TestGPOSMarkAttachmentLatinUntouched(t *testing.T) {
	// Build a Latin-capable mock face that also declares GPOS marks;
	// eligibility should still reject because the text has no Extend.
	face := newLamFathaFace()
	face.cmap['h'] = 1
	face.cmap['e'] = 2
	face.cmap['l'] = 3
	face.cmap['o'] = 4
	face.advance[1] = 500
	face.advance[2] = 500
	face.advance[3] = 500
	face.advance[4] = 500
	ef := font.NewEmbeddedFont(face)

	word := Word{
		Text:     "hello",
		Embedded: ef,
		FontSize: 12,
	}

	b := capturedWordStream(word)
	if countTdOps(b) != 1 {
		t.Errorf("Latin word should emit only the initial Td, got %d:\n%s", countTdOps(b), b)
	}
	if firstMarkTdBetween(b) != "" {
		t.Errorf("Latin word should not emit mark-Td brackets:\n%s", b)
	}
}

// TestGPOSMarkAttachmentTwoMarks verifies that a cluster with two
// Extend marks (fatha and shadda on the same lam) emits two separate
// Td-bracketed mark emissions, so each mark is positioned individually.
func TestGPOSMarkAttachmentTwoMarks(t *testing.T) {
	face := newLamFathaFace()
	const shaddaGID uint16 = 61
	face.cmap[0x0651] = shaddaGID // shadda
	face.advance[shaddaGID] = 0
	// Mark class 0 shared: fatha already uses class 0. Give shadda its
	// own class (class 1) so the base needs a second anchor slot. This
	// also exercises multi-class mark positioning.
	face.gpos.Marks[font.GPOSMark][shaddaGID] = font.MarkRecord{
		Class:  1,
		Anchor: font.Anchor{X: 100, Y: 400},
	}
	base := face.gpos.Bases[font.GPOSMark][50]
	base.Anchors = append(base.Anchors, font.Anchor{X: 500, Y: 900}) // class 1 anchor
	face.gpos.Bases[font.GPOSMark][50] = base

	ef := font.NewEmbeddedFont(face)
	word := Word{
		Text:     "\u0644\u064E\u0651", // lam + fatha + shadda
		Embedded: ef,
		FontSize: 12,
	}

	b := capturedWordStream(word)

	// Expect: base Tj, then two Td-bracketed mark emissions. That is:
	// base Tj, Td(open1), mark1 Tj, Td(close1), Td(open2), mark2 Tj, Td(close2).
	// Count Tjs and Tds.
	tjCount := 0
	tdCount := 0
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " Tj") {
			tjCount++
		}
		if strings.HasSuffix(line, " Td") {
			tdCount++
		}
	}
	if tjCount != 3 {
		t.Errorf("expected 3 Tj (base + 2 marks), got %d:\n%s", tjCount, b)
	}
	// Initial MoveText + two open Td + two close Td = 5.
	if tdCount != 5 {
		t.Errorf("expected 5 Td (initial + 2*(open+close)), got %d:\n%s", tdCount, b)
	}
}

// TestGPOSMarkAttachmentMeasureAgreesWithDraw is the correctness
// invariant: the width reported by EmbeddedFont.MeasureString for a
// mark-bearing word must equal the total horizontal advance the text
// matrix undergoes during drawWordEmbedded. The test simulates the
// matrix advance by parsing the content stream and summing Td x
// components plus the base Tj advance per cluster.
func TestGPOSMarkAttachmentMeasureAgreesWithDraw(t *testing.T) {
	face := newLamFathaFace()
	ef := font.NewEmbeddedFont(face)

	word := Word{
		Text:     "\u0644\u064E",
		Embedded: ef,
		FontSize: 12,
	}

	measured := ef.MeasureString(word.Text, word.FontSize)

	// The draw path advances the text matrix by the base's Tj advance
	// plus the net of all Td operators after the initial MoveText(0,0).
	// Reproduce that calculation directly.
	//
	// The only non-zero-advance glyph in the cluster is lam (700 FUnits
	// = 8.4 pt). Fatha is zero-advance. The Td bracket is matched pairs
	// (-4.8 +4.8 / +6 -6) which sum to zero. Net advance = 8.4 pt.
	// MeasureString should also report ~8.4 pt (modulo float rounding).
	want := 8.4
	if !almostEqual(measured, want, 1e-9) {
		t.Errorf("MeasureString: got %v, want %v", measured, want)
	}

	// Now parse the draw stream and sum Td advances (after the initial
	// move) plus base advance. This is a stand-in for running the PDF
	// through an interpreter.
	b := capturedWordStream(word)
	netTdX := 0.0
	seenInitial := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasSuffix(line, " Td") {
			continue
		}
		if !seenInitial {
			seenInitial = true // initial MoveText(0, 0) — ignore
			continue
		}
		var tx, ty float64
		n, err := fmt.Sscanf(line, "%f %f Td", &tx, &ty)
		if err != nil || n != 2 {
			t.Fatalf("unparseable Td line %q: %v", line, err)
		}
		netTdX += tx
	}
	// Base glyph Tj advance: one lam.
	baseAdv := float64(face.advance[50]) / float64(face.upem) * word.FontSize
	drawAdvance := baseAdv + netTdX
	if !almostEqual(drawAdvance, measured, 1e-9) {
		t.Errorf("draw advance = %v, MeasureString = %v — these must agree for line wrap/draw consistency", drawAdvance, measured)
	}
}

// addStackedShadda extends the lam+fatha fixture with a shadda (U+0651)
// that carries LookupType 6 data saying it stacks on top of the fatha
// rather than anchoring against the lam base. Mark1 (attaching) =
// shadda with anchor (100, 400); Mark2 (underlying) = fatha with
// anchor (150, 950) for class 0. LookupType 6 offset =
// (150-100, 950-400) = (50, 550) FUnits relative to the fatha's origin.
// Y differs from the mark-to-base offset (500 FUnits) so that any
// bug swapping mkmk for mark-to-base fails on both axes.
func addStackedShadda(face *mockGPOSFace) uint16 {
	const shaddaGID uint16 = 61
	face.cmap[0x0651] = shaddaGID
	face.advance[shaddaGID] = 0
	if face.gpos.MarkMarks == nil {
		face.gpos.MarkMarks = map[font.GPOSFeature]map[uint16]font.MarkRecord{}
	}
	if face.gpos.Mark2Bases == nil {
		face.gpos.Mark2Bases = map[font.GPOSFeature]map[uint16]font.BaseRecord{}
	}
	face.gpos.MarkMarks[font.GPOSMkmk] = map[uint16]font.MarkRecord{
		shaddaGID: {Class: 0, Anchor: font.Anchor{X: 100, Y: 400}},
	}
	face.gpos.Mark2Bases[font.GPOSMkmk] = map[uint16]font.BaseRecord{
		60: {Anchors: []font.Anchor{{X: 150, Y: 950}}}, // fatha GID 60
	}
	return shaddaGID
}

// TestGPOSMarkMarkAttachmentStacksSecondMark verifies that when mkmk
// data is present for the (mark2 = mark[i-1], mark1 = mark[i]) pair,
// the second mark's Td bracket opens at (prevDx + mkmkDx - clusterAdv,
// prevDy + mkmkDy) rather than at the plain mark-to-base offset. With
// the fixture above:
//
//	fatha origin (mark-to-base) = (300, 500) FUnits → (3.6, 6.0) pt
//	shadda origin (stacked)     = (300+50, 500+550) FUnits → (4.2, 12.6) pt
//	clusterAdvance = 8.4 pt
//
// First mark open Td:  (3.6 - 8.4, 6) = (-4.8, 6)
// Second mark open Td: (4.2 - 8.4, 12.6) = (-4.2, 12.6)
// Second mark close Td: (8.4 - 4.2, -12.6) = (4.2, -12.6)
func TestGPOSMarkMarkAttachmentStacksSecondMark(t *testing.T) {
	face := newLamFathaFace()
	_ = addStackedShadda(face)
	ef := font.NewEmbeddedFont(face)

	word := Word{
		Text:     "\u0644\u064E\u0651", // lam + fatha + shadda
		Embedded: ef,
		FontSize: 12,
	}
	b := capturedWordStream(word)

	// Collect every Td after the initial MoveText.
	var tds []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " Td") {
			tds = append(tds, line)
		}
	}
	// Initial + two open + two close = 5 Tds total.
	if len(tds) != 5 {
		t.Fatalf("expected 5 Td ops, got %d:\n%s", len(tds), b)
	}
	// tds[0]: MoveText(0, 0).
	// tds[1]: fatha open  -> "-4.8 6 Td"
	// tds[2]: fatha close -> "4.8 -6 Td"
	// tds[3]: shadda open (stacked) -> "-4.2 12.6 Td"
	// tds[4]: shadda close           -> "4.2 -12.6 Td"
	if !strings.Contains(tds[3], "-4.2 12.6 Td") {
		t.Errorf("shadda open Td = %q, want stacked offset -4.2 12.6 Td:\n%s", tds[3], b)
	}
	if !strings.Contains(tds[4], "4.2 -12.6 Td") {
		t.Errorf("shadda close Td = %q, want 4.2 -12.6 Td:\n%s", tds[4], b)
	}
}

// TestGPOSMarkMarkAttachmentThreeStackedMarks exercises transitive
// stacking: a fourth glyph (dammatan, U+064C) stacks on shadda, which
// itself stacks on fatha, which anchors to lam. This confirms that
// prevDxPts/prevDyPts accumulate across marks rather than collapsing
// to the previous mark's mkmk delta only.
//
//	lam      class-0 anchor (500, 800)
//	fatha    class 0, anchor (200, 300)   → origin (300, 500) = (3.6, 6.0)
//	shadda   mkmk class 0 on fatha's (150, 950) with mark1 anchor
//	         (100, 400) → delta (50, 550) → origin (350, 1050) = (4.2, 12.6)
//	dammatan mkmk class 0 on shadda's (120, 1100) with mark1 anchor
//	         (80, 500) → delta (40, 600) → origin (390, 1650) = (4.68, 19.8)
func TestGPOSMarkMarkAttachmentThreeStackedMarks(t *testing.T) {
	face := newLamFathaFace()
	_ = addStackedShadda(face)
	const dammatanGID uint16 = 62
	face.cmap[0x064C] = dammatanGID // dammatan
	face.advance[dammatanGID] = 0
	face.gpos.MarkMarks[font.GPOSMkmk][dammatanGID] = font.MarkRecord{
		Class:  0,
		Anchor: font.Anchor{X: 80, Y: 500},
	}
	face.gpos.Mark2Bases[font.GPOSMkmk][61 /* shadda */] = font.BaseRecord{
		Anchors: []font.Anchor{{X: 120, Y: 1100}},
	}

	ef := font.NewEmbeddedFont(face)
	word := Word{
		Text:     "\u0644\u064E\u0651\u064C", // lam + fatha + shadda + dammatan
		Embedded: ef,
		FontSize: 12,
	}
	b := capturedWordStream(word)

	var tds []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " Td") {
			tds = append(tds, line)
		}
	}
	// Initial + three open + three close = 7 Tds total.
	if len(tds) != 7 {
		t.Fatalf("expected 7 Td ops (initial + 3*(open+close)), got %d:\n%s", len(tds), b)
	}
	// tds[5]: dammatan open  -> want "-3.72 19.8 Td"
	// tds[6]: dammatan close -> want "3.72 -19.8 Td"
	if !strings.Contains(tds[5], "-3.72 19.8 Td") {
		t.Errorf("dammatan open Td = %q, want -3.72 19.8 Td (prevDx + mkmkDx accumulation):\n%s", tds[5], b)
	}
	if !strings.Contains(tds[6], "3.72 -19.8 Td") {
		t.Errorf("dammatan close Td = %q, want 3.72 -19.8 Td:\n%s", tds[6], b)
	}
	// Open/close pairs must sum to zero on both axes.
	var netX, netY float64
	for i, td := range tds {
		if i == 0 {
			continue // initial MoveText(0, 0)
		}
		var tx, ty float64
		n, err := fmt.Sscanf(td, "%f %f Td", &tx, &ty)
		if err != nil || n != 2 {
			t.Fatalf("unparseable Td %q: %v", td, err)
		}
		netX += tx
		netY += ty
	}
	if !almostEqual(netX, 0, 1e-9) || !almostEqual(netY, 0, 1e-9) {
		t.Errorf("Td bracket pairs must cancel; net = (%v, %v)", netX, netY)
	}
}

// TestGPOSMarkMarkFallbackWithWrongGID verifies the mkmk miss path is
// keyed by glyph ID, not by feature-presence. The font carries mkmk
// data for a *different* mark1 GID than the one being placed; the
// current mark must fall back to mark-to-base against the cluster
// base, not silently reuse whatever mkmk state is nearby.
func TestGPOSMarkMarkFallbackWithWrongGID(t *testing.T) {
	face := newLamFathaFace()
	const shaddaGID uint16 = 61
	face.cmap[0x0651] = shaddaGID
	face.advance[shaddaGID] = 0
	// Shadda has mark-to-base data sharing class 0 with fatha, so lam's
	// existing class-0 anchor positions it.
	face.gpos.Marks[font.GPOSMark][shaddaGID] = font.MarkRecord{
		Class:  0,
		Anchor: font.Anchor{X: 200, Y: 300},
	}
	// Populate mkmk, but only for an unrelated GID 99 as mark1 — the
	// actual shadda glyph (GID 61) must miss.
	face.gpos.MarkMarks = map[font.GPOSFeature]map[uint16]font.MarkRecord{
		font.GPOSMkmk: {99: {Class: 0, Anchor: font.Anchor{X: 100, Y: 400}}},
	}
	face.gpos.Mark2Bases = map[font.GPOSFeature]map[uint16]font.BaseRecord{
		font.GPOSMkmk: {60: {Anchors: []font.Anchor{{X: 150, Y: 950}}}},
	}

	ef := font.NewEmbeddedFont(face)
	word := Word{
		Text:     "\u0644\u064E\u0651",
		Embedded: ef,
		FontSize: 12,
	}
	b := capturedWordStream(word)

	var tds []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " Td") {
			tds = append(tds, line)
		}
	}
	if len(tds) != 5 {
		t.Fatalf("expected 5 Td ops, got %d:\n%s", len(tds), b)
	}
	// Shadda must land at mark-to-base offset (-4.8, 6), not at the
	// mkmk-stacked offset (-4.2, 12.6).
	if !strings.Contains(tds[3], "-4.8 6 Td") {
		t.Errorf("shadda open Td = %q, want mark-to-base fallback -4.8 6 Td (wrong-GID mkmk miss):\n%s", tds[3], b)
	}
}

// TestGPOSMarkMarkFallsBackWhenPairUnknown verifies that when LookupType
// 6 has no entry for (mark[i], mark[i-1]), the mark falls back to
// mark-to-base against the cluster base. Here shadda declares a
// mark-to-base anchor (class 0 on lam) but no mkmk relation to fatha:
// the shadda must land at its mark-to-base offset, not stacked.
func TestGPOSMarkMarkFallsBackWhenPairUnknown(t *testing.T) {
	face := newLamFathaFace()
	const shaddaGID uint16 = 61
	face.cmap[0x0651] = shaddaGID
	face.advance[shaddaGID] = 0
	// Give shadda a mark-to-base record sharing class 0 so lam's
	// existing class-0 anchor serves it. Expected offset equals the
	// fatha offset: (300, 500) FUnits → (3.6, 6.0) pt.
	face.gpos.Marks[font.GPOSMark][shaddaGID] = font.MarkRecord{
		Class:  0,
		Anchor: font.Anchor{X: 200, Y: 300},
	}
	// No MarkMarks / Mark2Bases populated: mkmk lookup must miss.

	ef := font.NewEmbeddedFont(face)
	word := Word{
		Text:     "\u0644\u064E\u0651",
		Embedded: ef,
		FontSize: 12,
	}
	b := capturedWordStream(word)

	var tds []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " Td") {
			tds = append(tds, line)
		}
	}
	if len(tds) != 5 {
		t.Fatalf("expected 5 Td ops, got %d:\n%s", len(tds), b)
	}
	// Both marks should land at the same (-4.8, 6) open because both
	// resolve to the lam class-0 anchor via mark-to-base fallback.
	if !strings.Contains(tds[3], "-4.8") || !strings.Contains(tds[3], "6 Td") {
		t.Errorf("fallback shadda open Td = %q, want mark-to-base -4.8 6 Td:\n%s", tds[3], b)
	}
}

// TestGPOSMarkMarkMeasureAgreesWithDraw locks down the invariant for
// stacked marks: mkmk positioning is purely visual (zero-advance marks,
// paired Td brackets that cancel), so MeasureString must still equal
// the net horizontal advance of the draw stream.
func TestGPOSMarkMarkMeasureAgreesWithDraw(t *testing.T) {
	face := newLamFathaFace()
	_ = addStackedShadda(face)
	ef := font.NewEmbeddedFont(face)

	word := Word{
		Text:     "\u0644\u064E\u0651",
		Embedded: ef,
		FontSize: 12,
	}
	measured := ef.MeasureString(word.Text, word.FontSize)

	b := capturedWordStream(word)
	netTdX := 0.0
	seenInitial := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasSuffix(line, " Td") {
			continue
		}
		if !seenInitial {
			seenInitial = true
			continue
		}
		var tx, ty float64
		n, err := fmt.Sscanf(line, "%f %f Td", &tx, &ty)
		if err != nil || n != 2 {
			t.Fatalf("unparseable Td line %q: %v", line, err)
		}
		netTdX += tx
	}
	baseAdv := float64(face.advance[50]) / float64(face.upem) * word.FontSize
	drawAdvance := baseAdv + netTdX
	if !almostEqual(drawAdvance, measured, 1e-9) {
		t.Errorf("mkmk draw advance = %v, MeasureString = %v — must agree", drawAdvance, measured)
	}
}

func almostEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}
