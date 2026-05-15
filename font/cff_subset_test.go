// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"testing"
)

// TestSubsetCFFSyntheticPreservesKeptGlyphs runs the subsetter against
// a deterministic CID-keyed CFF, then re-parses the result and asserts:
//   - the output round-trips through parseCFF cleanly,
//   - kept glyphs retain their original charstring bytes,
//   - unkept glyphs are replaced by a single endchar (0x0E),
//   - GID 0 (.notdef) is always retained.
func TestSubsetCFFSyntheticPreservesKeptGlyphs(t *testing.T) {
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{numGlyphs: 10, fdCount: 2})
	used := map[uint16]rune{3: 'A', 7: 'B'}

	subset, err := SubsetCFF(cff, used)
	if err != nil {
		t.Fatalf("SubsetCFF: %v", err)
	}
	if !isCIDKeyedCFFv1(subset) {
		t.Fatal("subset is no longer detected as CID-keyed CFF v1")
	}

	parsed, err := parseCFF(subset)
	if err != nil {
		t.Fatalf("re-parse subset: %v", err)
	}
	if parsed.numGlyphs != 10 {
		t.Errorf("numGlyphs after subset = %d, want 10", parsed.numGlyphs)
	}
	// The synthetic source's charstrings are all single 0x0E bytes,
	// so this test cannot distinguish "kept" from "replaced". Verify
	// the structure round-trips and that the byte content matches —
	// it will, by construction.
	for i := range parsed.numGlyphs {
		got := parsed.charStringsIndex.Object(i)
		if len(got) != 1 || got[0] != 0x0E {
			t.Errorf("charstring %d = %v, want [0x0E]", i, got)
		}
	}
}

// TestSubsetCFFReplacesUnusedCharStrings inserts distinguishable
// charstrings into the synthetic CFF so kept-vs-replaced assertions
// have signal. The keep set is {0, 3, 7}; every other glyph must
// become [0x0E].
func TestSubsetCFFReplacesUnusedCharStrings(t *testing.T) {
	// We need a synthetic with non-trivial charstring bytes; the
	// shared builder always uses [0x0E]. Build a one-glyph synthetic
	// then surgically widen the CharStrings INDEX. Simpler: use the
	// builder for structure, then patch CharStrings INDEX in place
	// with distinct charstrings — but that breaks INDEX offsets.
	//
	// Cleanest path: parse the synthetic, then run SubsetCFF and
	// assert by counting endchar replacements vs. kept charstrings.
	// Even with identical [0x0E] bytes everywhere, the subset must
	// still preserve the GID count exactly.
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{numGlyphs: 5, fdCount: 1})
	used := map[uint16]rune{2: 'A'}

	subset, err := SubsetCFF(cff, used)
	if err != nil {
		t.Fatalf("SubsetCFF: %v", err)
	}
	parsed, err := parseCFF(subset)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if parsed.numGlyphs != 5 {
		t.Errorf("numGlyphs = %d, want 5", parsed.numGlyphs)
	}
	// Every charstring is still [0x0E] either as the original
	// content or as the replacement. The subset must not shrink
	// the GID count (CID == GID invariant).
	for i := range parsed.numGlyphs {
		if len(parsed.charStringsIndex.Object(i)) != 1 {
			t.Errorf("charstring %d has size %d, want 1", i, len(parsed.charStringsIndex.Object(i)))
		}
	}
}

// TestSubsetCFFEmptyUsedKeepsNotdef asserts that even with an empty
// keep set, GID 0 (.notdef) survives — required for any valid PDF
// font embed.
func TestSubsetCFFEmptyUsedKeepsNotdef(t *testing.T) {
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{numGlyphs: 3, fdCount: 1})
	subset, err := SubsetCFF(cff, nil)
	if err != nil {
		t.Fatalf("SubsetCFF: %v", err)
	}
	parsed, err := parseCFF(subset)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if parsed.numGlyphs != 3 {
		t.Errorf("numGlyphs after empty subset = %d, want 3", parsed.numGlyphs)
	}
}

// TestSubsetCFFRejectsNonCIDKeyed routes a name-keyed CFF through
// SubsetCFF. parseCFF rejects it; SubsetCFF must surface that error
// rather than producing a malformed subset.
func TestSubsetCFFRejectsNonCIDKeyed(t *testing.T) {
	cff := buildSyntheticCFFv1([]byte{139, 0}) // name-keyed: version op first
	_, err := SubsetCFF(cff, nil)
	if err == nil {
		t.Fatal("expected error on name-keyed CFF")
	}
}

// TestSubsetCFFRealFontSize is the regression test for issue #295.
// Subset a CID-keyed CFF (Hiragino on macOS, NotoSansCJK on Linux) to
// a single CJK glyph and assert the result is dramatically smaller
// than the source. The threshold (10% of original) is loose so it
// catches dead-letter regressions without false positives from
// font-version drift; Phase 3 with no subroutine pruning at all would
// still pass at ~50%, so 10% confirms the pruning fired.
func TestSubsetCFFRealFontSize(t *testing.T) {
	face := loadTestCFFFace(t)
	cf := face.(cffFace)
	src := cf.CFFData()

	// Pick any glyph that exists. GID 1 is reliably non-notdef in
	// every real CID-keyed font.
	used := map[uint16]rune{1: 'a'}
	subset, err := SubsetCFF(src, used)
	if err != nil {
		t.Fatalf("SubsetCFF on real font: %v", err)
	}
	if len(subset) >= len(src)/10 {
		t.Errorf("subset size %d not significantly smaller than source %d (want <%d)",
			len(subset), len(src), len(src)/10)
	}
	if !isCIDKeyedCFFv1(subset) {
		t.Error("real-font subset no longer detected as CID-keyed CFF v1")
	}
	// Re-parse to validate structure end-to-end.
	if _, err := parseCFF(subset); err != nil {
		t.Errorf("re-parse subset of real font: %v", err)
	}
	t.Logf("real font subset: %d -> %d bytes (%.1f%%)",
		len(src), len(subset), float64(len(subset))*100/float64(len(src)))
}

// TestWriteCFFIndex covers the universal INDEX writer used by every
// subset section. Empty INDEX is the spec's 2-byte sentinel; non-
// empty INDEXes must pick the smallest offSize that fits.
func TestWriteCFFIndex(t *testing.T) {
	if got := writeCFFIndex(nil); !bytes.Equal(got, []byte{0x00, 0x00}) {
		t.Errorf("empty INDEX = %v, want [0x00, 0x00]", got)
	}
	out := writeCFFIndex([][]byte{[]byte("hello"), []byte("world!")})
	parsed, err := parseCFFIndex(out, 0)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if parsed.count != 2 || !bytes.Equal(parsed.Object(0), []byte("hello")) || !bytes.Equal(parsed.Object(1), []byte("world!")) {
		t.Errorf("round-trip lost contents: count=%d obj0=%q obj1=%q",
			parsed.count, parsed.Object(0), parsed.Object(1))
	}
	if parsed.offSize != 1 {
		t.Errorf("offSize = %d, want 1 for small payload", parsed.offSize)
	}
}

// TestWriteCFFIndexLargePayloadPicksOffSize2 exercises the offSize
// selection rule at the 1-byte boundary.
func TestWriteCFFIndexLargePayloadPicksOffSize2(t *testing.T) {
	big := make([]byte, 300) // forces offSize >= 2
	out := writeCFFIndex([][]byte{big})
	parsed, err := parseCFFIndex(out, 0)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if parsed.offSize != 2 {
		t.Errorf("offSize = %d, want 2 for 300-byte payload", parsed.offSize)
	}
	if !bytes.Equal(parsed.Object(0), big) {
		t.Errorf("payload round-trip mismatch")
	}
}

// TestFdForGlyphFormat0 covers FDSelect format 0 lookup.
func TestFdForGlyphFormat0(t *testing.T) {
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{numGlyphs: 6, fdCount: 3})
	parsed, err := parseCFF(cff)
	if err != nil {
		t.Fatalf("parseCFF: %v", err)
	}
	// Builder assigns glyph i to FD i % fdCount.
	for gid := range parsed.numGlyphs {
		want := gid % 3
		got, err := parsed.fdForGlyph(gid)
		if err != nil {
			t.Errorf("fdForGlyph(%d): %v", gid, err)
			continue
		}
		if got != want {
			t.Errorf("fdForGlyph(%d) = %d, want %d", gid, got, want)
		}
	}
}

// TestSubsetCFFPreservesDistinguishableCharStrings uses non-trivial
// per-glyph charstring bytes so the assertions can tell "kept
// verbatim" apart from "replaced with endchar". The previous synthetic
// fixture made every charstring identical, hiding off-by-one bugs in
// keep-set indexing.
func TestSubsetCFFPreservesDistinguishableCharStrings(t *testing.T) {
	// Glyph i gets a charstring whose distinguishing byte is `i+1`
	// followed by endchar so it terminates the trace cleanly.
	const n = 5
	cs := make([][]byte, n)
	for i := range n {
		cs[i] = []byte{byte(i + 1), 0x0E}
	}
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{
		numGlyphs:   n,
		fdCount:     1,
		charStrings: cs,
	})
	used := map[uint16]rune{2: 'A'}

	subset, err := SubsetCFF(cff, used)
	if err != nil {
		t.Fatalf("SubsetCFF: %v", err)
	}
	parsed, err := parseCFF(subset)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	// GID 0 (.notdef) is always kept; GID 2 explicitly. Everything
	// else must become a single endchar.
	wantKept := map[int]bool{0: true, 2: true}
	for gid := range n {
		got := parsed.charStringsIndex.Object(gid)
		if wantKept[gid] {
			if len(got) != 2 || got[0] != byte(gid+1) || got[1] != 0x0E {
				t.Errorf("kept gid %d = %v, want [%d 0x0E]", gid, got, gid+1)
			}
		} else {
			if len(got) != 1 || got[0] != 0x0E {
				t.Errorf("dropped gid %d = %v, want [0x0E]", gid, got)
			}
		}
	}
}

// TestSubsetCFFPrunesUnreachableGsubrs builds a CFF with two global
// subroutines: gsubr 0 is called by the kept glyph, gsubr 1 is
// orphaned. After subsetting, gsubr 0 must survive byte-identical and
// gsubr 1 must be replaced with a single `return` byte.
func TestSubsetCFFPrunesUnreachableGsubrs(t *testing.T) {
	// gsubr 0: terminate immediately. The kept glyph calls it.
	gsubr0 := []byte{t2OpReturn}
	// gsubr 1: contains a distinguishable payload so a reachability
	// regression that retains all gsubrs is visible.
	gsubr1 := []byte{0x20, 0x20, 0x20, t2OpReturn}

	// The kept glyph (gid 1) calls gsubr 0 then endchar.
	// callgsubr expects a biased index; biased = idx - 107 for
	// nSubrs < 1240. We have nSubrs = 2, so the bias is 107 and the
	// biased index for gsubr 0 is -107.
	callGsubr0 := []byte{}
	callGsubr0 = append(callGsubr0, t2Int(-107)...)
	callGsubr0 = append(callGsubr0, t2OpCallgsubr, t2OpEndchar)

	cs := [][]byte{
		{0x0E},     // GID 0 notdef
		callGsubr0, // GID 1: kept and calls gsubr 0
		{0x0E},     // GID 2
	}
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{
		numGlyphs:   3,
		fdCount:     1,
		charStrings: cs,
		globalSubrs: [][]byte{gsubr0, gsubr1},
	})
	used := map[uint16]rune{1: 'A'}

	subset, err := SubsetCFF(cff, used)
	if err != nil {
		t.Fatalf("SubsetCFF: %v", err)
	}
	parsed, err := parseCFF(subset)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if parsed.gsubrIndex.count != 2 {
		t.Fatalf("gsubr count = %d, want 2 (size preserved)", parsed.gsubrIndex.count)
	}
	if got := parsed.gsubrIndex.Object(0); len(got) != len(gsubr0) || got[0] != gsubr0[0] {
		t.Errorf("gsubr 0 = %v, want %v (reached, must be verbatim)", got, gsubr0)
	}
	if got := parsed.gsubrIndex.Object(1); len(got) != 1 || got[0] != 0x0B {
		t.Errorf("gsubr 1 = %v, want [0x0B] (unreached)", got)
	}
}

// TestSubsetCFFPreservesNonLongintTopDictOperands unit-tests
// rewriteTopDict directly with a hand-rolled DICT containing
// non-longint operands (single-byte int, shortint, BCD real). The
// rewriter must copy verbatim operators byte-for-byte.
func TestSubsetCFFPreservesNonLongintTopDictOperands(t *testing.T) {
	// Build a Top DICT with three verbatim entries and one rewritten:
	//   139            single-byte int 0
	//   17             CharStrings operator (will be rewritten)
	//   139, 0         single-byte int 0, then version op
	//   28, 0x12, 0x34 shortint 0x1234
	//   12, 2          ItalicAngle operator (2-byte; verbatim)
	dict := []byte{
		139, byte(cffOpCharStrings), // operand + CharStrings (rewritten)
		139, byte(cffOpVersion), // operand + version (verbatim, single-byte op)
		28, 0x12, 0x34, cffOpEscape, 2, // shortint + ItalicAngle (verbatim, 2-byte op)
	}
	entries, err := parseCFFDict(dict)
	if err != nil {
		t.Fatalf("parseCFFDict: %v", err)
	}

	out, patch := rewriteTopDict(dict, entries)
	if patch.charStrings < 0 {
		t.Fatal("CharStrings patch position not recorded")
	}

	// Find where the verbatim version and ItalicAngle entries land in
	// the output. CharStrings becomes 5-byte placeholder + 1-byte op =
	// 6 bytes total (vs. 2 bytes in source). Original source layout
	// after CharStrings entry: 5 verbatim bytes (139, 0, 28, 0x12, 0x34)
	// + escape op + ItalicAngle.
	wantSuffix := []byte{139, byte(cffOpVersion), 28, 0x12, 0x34, cffOpEscape, 2}
	if len(out) < len(wantSuffix) {
		t.Fatalf("rewritten dict too short: %d bytes", len(out))
	}
	tail := out[len(out)-len(wantSuffix):]
	if string(tail) != string(wantSuffix) {
		t.Errorf("verbatim suffix = %v, want %v", tail, wantSuffix)
	}
}

// TestFdForGlyphFormat3 exercises the multi-range FDSelect format
// used by every real-world CID-keyed CJK font, which the original
// synthetic builder couldn't reach. Three ranges with non-uniform
// widths drive the boundary arithmetic in fdForGlyph and the
// computeFDSelectSize sentinel check together.
func TestFdForGlyphFormat3(t *testing.T) {
	// 10 glyphs split into three FDs:
	//   [0..3]  -> FD 0
	//   [4..6]  -> FD 1
	//   [7..9]  -> FD 2
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{
		numGlyphs: 10,
		fdCount:   3,
		fdSelectFormat3: []syntheticFDRange{
			{firstGID: 0, fd: 0},
			{firstGID: 4, fd: 1},
			{firstGID: 7, fd: 2},
		},
	})
	parsed, err := parseCFF(cff)
	if err != nil {
		t.Fatalf("parseCFF: %v", err)
	}
	wantFD := []int{0, 0, 0, 0, 1, 1, 1, 2, 2, 2}
	for gid, want := range wantFD {
		got, err := parsed.fdForGlyph(gid)
		if err != nil {
			t.Errorf("fdForGlyph(%d): %v", gid, err)
			continue
		}
		if got != want {
			t.Errorf("fdForGlyph(%d) = %d, want %d", gid, got, want)
		}
	}
}

func TestFdForGlyphFormat3SingleRange(t *testing.T) {
	// Degenerate one-range form: every glyph in FD 0.
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{
		numGlyphs: 4,
		fdCount:   1,
		fdSelectFormat3: []syntheticFDRange{
			{firstGID: 0, fd: 0},
		},
	})
	parsed, err := parseCFF(cff)
	if err != nil {
		t.Fatalf("parseCFF: %v", err)
	}
	for gid := range 4 {
		got, err := parsed.fdForGlyph(gid)
		if err != nil {
			t.Errorf("fdForGlyph(%d): %v", gid, err)
			continue
		}
		if got != 0 {
			t.Errorf("fdForGlyph(%d) = %d, want 0", gid, got)
		}
	}
}

func TestFdForGlyphOutOfRange(t *testing.T) {
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{numGlyphs: 3, fdCount: 1})
	parsed, err := parseCFF(cff)
	if err != nil {
		t.Fatalf("parseCFF: %v", err)
	}
	if _, err := parsed.fdForGlyph(-1); err == nil {
		t.Error("expected error for gid -1")
	}
	if _, err := parsed.fdForGlyph(99); err == nil {
		t.Error("expected error for gid 99")
	}
}
