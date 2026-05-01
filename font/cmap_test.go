// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"errors"
	"os"
	"runtime"
	"testing"
)

// buildFormat4Subtable serialises a minimal format-4 subtable using
// idDelta-only segments (no idRangeOffset glyph array). Each segment
// is (start, end, delta). The terminating 0xFFFF segment is appended
// automatically.
func buildFormat4Subtable(segments []struct {
	start, end uint16
	delta      int16
}) []byte {
	segs := append([]struct {
		start, end uint16
		delta      int16
	}{}, segments...)
	segs = append(segs, struct {
		start, end uint16
		delta      int16
	}{0xFFFF, 0xFFFF, 1})
	segCount := len(segs)
	length := 14 + segCount*2*4 + 2 // 4 arrays + reservedPad
	buf := make([]byte, length)
	binary.BigEndian.PutUint16(buf[0:], 4)
	binary.BigEndian.PutUint16(buf[2:], uint16(length))
	binary.BigEndian.PutUint16(buf[4:], 0) // language
	binary.BigEndian.PutUint16(buf[6:], uint16(segCount*2))
	// search range, entry selector, range shift not strictly required by parser.
	off := 14
	for _, s := range segs {
		binary.BigEndian.PutUint16(buf[off:], s.end)
		off += 2
	}
	off += 2 // reservedPad
	for _, s := range segs {
		binary.BigEndian.PutUint16(buf[off:], s.start)
		off += 2
	}
	for _, s := range segs {
		binary.BigEndian.PutUint16(buf[off:], uint16(s.delta))
		off += 2
	}
	// idRangeOffset all zeros.
	return buf
}

// buildFormat12Subtable serialises a format-12 subtable from a slice
// of (startChar, endChar, startGID) groups.
func buildFormat12Subtable(groups []struct{ startChar, endChar, startGID uint32 }) []byte {
	length := 16 + len(groups)*12
	buf := make([]byte, length)
	binary.BigEndian.PutUint16(buf[0:], 12)
	// reserved at [2:4] = 0
	binary.BigEndian.PutUint32(buf[4:], uint32(length))
	binary.BigEndian.PutUint32(buf[8:], 0) // language
	binary.BigEndian.PutUint32(buf[12:], uint32(len(groups)))
	for i, g := range groups {
		off := 16 + i*12
		binary.BigEndian.PutUint32(buf[off:], g.startChar)
		binary.BigEndian.PutUint32(buf[off+4:], g.endChar)
		binary.BigEndian.PutUint32(buf[off+8:], g.startGID)
	}
	return buf
}

// wrapCmap wraps subtables (each with platformID/encodingID) into a
// complete cmap table. Returns the final cmap bytes ready for
// parseCmapTable.
func wrapCmap(records []struct {
	platformID, encodingID uint16
	subtable               []byte
}) []byte {
	header := 4 + len(records)*8
	// Lay out subtables back-to-back after the header.
	totalLen := header
	offsets := make([]int, len(records))
	for i, r := range records {
		offsets[i] = totalLen
		totalLen += len(r.subtable)
	}
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint16(buf[0:], 0)
	binary.BigEndian.PutUint16(buf[2:], uint16(len(records)))
	for i, r := range records {
		off := 4 + i*8
		binary.BigEndian.PutUint16(buf[off:], r.platformID)
		binary.BigEndian.PutUint16(buf[off+2:], r.encodingID)
		binary.BigEndian.PutUint32(buf[off+4:], uint32(offsets[i]))
		copy(buf[offsets[i]:], r.subtable)
	}
	return buf
}

func TestParseCmapFormat4Single(t *testing.T) {
	sub := buildFormat4Subtable([]struct {
		start, end uint16
		delta      int16
	}{
		{start: 0x41, end: 0x5A, delta: -0x40}, // 'A'..'Z' → GID 1..26
	})
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 3, encodingID: 1, subtable: sub},
	})
	tab, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable: %v", err)
	}
	if tab['A'] != 1 || tab['Z'] != 26 {
		t.Errorf("expected A→1, Z→26; got A=%d Z=%d", tab['A'], tab['Z'])
	}
	if _, ok := tab[0xFFFF]; ok {
		t.Error("sentinel segment should not appear in resolved table")
	}
}

func TestParseCmapFormat12Single(t *testing.T) {
	sub := buildFormat12Subtable([]struct{ startChar, endChar, startGID uint32 }{
		{startChar: 0x4E00, endChar: 0x4E0F, startGID: 1000},
	})
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 0, encodingID: 4, subtable: sub},
	})
	tab, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable: %v", err)
	}
	if tab[0x4E00] != 1000 || tab[0x4E0F] != 1015 {
		t.Errorf("CJK segment lookup wrong: 0x4E00=%d 0x4E0F=%d", tab[0x4E00], tab[0x4E0F])
	}
}

func TestParseCmapPrefersFormat12OverFormat4(t *testing.T) {
	sub4 := buildFormat4Subtable([]struct {
		start, end uint16
		delta      int16
	}{
		{start: 'A', end: 'A', delta: 99 - 'A'},
	})
	sub12 := buildFormat12Subtable([]struct{ startChar, endChar, startGID uint32 }{
		{startChar: 'A', endChar: 'A', startGID: 7},
	})
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 3, encodingID: 1, subtable: sub4},  // format 4
		{platformID: 0, encodingID: 4, subtable: sub12}, // format 12
	})
	tab, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable: %v", err)
	}
	if tab['A'] != 7 {
		t.Errorf("format 12 should win; got A=%d, want 7", tab['A'])
	}
}

// TestParseCmapFormat12LargeGroupCount stresses the parser with 25k
// groups, exceeding the hard-coded 20000-segment limit that
// golang.org/x/image/font/sfnt enforced. CJK fonts in the wild
// (Microsoft YaHei, Noto Sans CJK, STHeiti) routinely have tens of
// thousands of groups.
//
// This test is the load-bearing regression pin for issue #248: a stub
// parseCmapFormat12 that returned an empty map would still satisfy
// the type signature, so we explicitly assert a non-zero entry from
// the synthetic groups round-trips back.
func TestParseCmapFormat12LargeGroupCount(t *testing.T) {
	const N = 25000
	groups := make([]struct{ startChar, endChar, startGID uint32 }, N)
	for i := range N {
		// Place each group in the supplementary plane to avoid colliding
		// with format-4 sentinel runes; one rune per group keeps the
		// fixture small.
		c := uint32(0x10000 + i)
		groups[i] = struct{ startChar, endChar, startGID uint32 }{
			startChar: c,
			endChar:   c,
			startGID:  uint32(i + 1),
		}
	}
	sub := buildFormat12Subtable(groups)
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 0, encodingID: 4, subtable: sub},
	})
	tab, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable on %d-group fixture: %v", N, err)
	}
	if got := tab[0x10000]; got != 1 {
		t.Errorf("first group: tab[0x10000]=%d, want 1", got)
	}
	if got := tab[rune(0x10000+N-1)]; got != uint16(N&0xFFFF) {
		// Note: GID is uint16 — 25000 still fits, no truncation expected here.
		t.Errorf("last group: tab[0x%X]=%d, want %d", 0x10000+N-1, got, N)
	}
}

func TestParseCmapTruncatedHeader(t *testing.T) {
	_, err := parseCmapTable([]byte{0, 0, 0})
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want errors.Is ErrTruncated", err)
	}
}

func TestParseCmapNoSupportedSubtable(t *testing.T) {
	// One subtable, format 13 (unsupported here).
	sub := []byte{0, 13, 0, 0}
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 3, encodingID: 1, subtable: sub},
	})
	_, err := parseCmapTable(cmap)
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want errors.Is ErrCorruptTable", err)
	}
}

// TestParseCmapSTHeitiOpportunistic loads /System/Library/Fonts/STHeiti
// Light.ttc on darwin and verifies that 中 (U+4E2D) maps to a
// non-zero glyph via the Face surface. STHeiti is the user-reported
// font from issue #227 that triggered the cmap-segment-limit failure
// in golang.org/x/image/font/sfnt; this test pins the regression.
func TestParseCmapSTHeitiOpportunistic(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("STHeiti is a darwin-only system font")
	}
	const path = "/System/Library/Fonts/STHeiti Light.ttc"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("STHeiti not available: %v", err)
	}
	face, err := ParseFont(data)
	if err != nil {
		t.Fatalf("ParseFont(STHeiti): %v — this is exactly the issue #227 failure we are pinning", err)
	}
	if gid := face.GlyphIndex('中'); gid == 0 {
		t.Errorf("STHeiti: GlyphIndex('中') = 0, expected non-zero (font is supposed to cover Han)")
	}
}
