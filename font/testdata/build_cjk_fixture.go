// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

// Build script for font/testdata/synthetic_cjk.ttf — a small,
// purpose-built TrueType font covering exactly the CJK codepoints
// the integration test exercises (issue #281). The .ttf is checked
// into the repo so the test runs on every host without needing a
// system CJK font installed.
//
// Run from the repository root:
//
//	go run ./font/testdata/build_cjk_fixture.go
//
// The output is a ~1 KB sfnt with the bare-minimum tables Folio's
// parser, layout, embedder, and reader all need. Each glyph is an
// empty simple-glyph (numberOfContours == 0) so the font has no
// visual rendering; the test only checks text round-trip via the
// /ToUnicode CMap, not visual output.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

// glyphMap defines the codepoint → glyph ID assignment. GID 0 is
// reserved for .notdef; the actual CJK glyphs start at GID 1.
//
// The runes here mirror the integration test's phrase
//
//	中华人民共和国是一个历史悠久的文明古国。
//
// in font/cjk_drop_sfnt_roundtrip_test.go. Keep them in sync; if
// the test phrase changes, regenerate the fixture.
var codepoints = []rune{
	'中', '华', '人', '民', '共', '和', '国', '是', '一', '个',
	'历', '史', '悠', '久', '的', '文', '明', '古',
	'。',
}

const (
	unitsPerEm = 1000
	advance    = 1000 // square em advance for every glyph; metrics-only
)

func main() {
	numGlyphs := len(codepoints) + 1 // +1 for .notdef

	tables := map[string][]byte{
		"head": buildHead(),
		"hhea": buildHhea(numGlyphs),
		"maxp": buildMaxp(numGlyphs),
		"cmap": buildCmap(codepoints),
		"name": buildName(),
		"hmtx": buildHmtx(numGlyphs),
		"glyf": buildGlyf(numGlyphs),
		"loca": buildLoca(numGlyphs),
		"post": buildPost(),
	}

	ttf := assembleTTF(tables)

	out := "font/testdata/synthetic_cjk.ttf"
	if err := os.WriteFile(out, ttf, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes, %d glyphs)\n", out, len(ttf), numGlyphs)
}

// buildHead emits the 54-byte head table. The values that matter to
// downstream consumers are unitsPerEm (font design units), bbox
// fields (FontBBox in the PDF FontDescriptor), and indexToLocFormat
// (which we set to 1 / long-offset format so the loca builder can use
// uint32 offsets). checksumAdjustment is left at 0; Folio's parser
// does not verify it and validating PDF readers do not look at the
// embedded sfnt header. Created/Modified are fixed values for
// deterministic build output.
func buildHead() []byte {
	buf := new(bytes.Buffer)
	write32(buf, 0x00010000) // version 1.0
	write32(buf, 0x00010000) // fontRevision 1.0
	write32(buf, 0)          // checkSumAdjustment (ignored)
	write32(buf, 0x5F0F3CF5) // magicNumber
	write16(buf, 0)          // flags
	write16(buf, unitsPerEm) // unitsPerEm
	write64(buf, 0)          // created
	write64(buf, 0)          // modified
	writeI16(buf, 0)         // xMin
	writeI16(buf, -200)      // yMin
	writeI16(buf, advance)   // xMax
	writeI16(buf, 800)       // yMax
	write16(buf, 0)          // macStyle
	write16(buf, 8)          // lowestRecPPEM
	writeI16(buf, 2)         // fontDirectionHint (deprecated; 2 = mixed)
	writeI16(buf, 1)         // indexToLocFormat (1 = long uint32 offsets)
	writeI16(buf, 0)         // glyphDataFormat
	return buf.Bytes()
}

// buildHhea emits the 36-byte horizontal header. ascender/descender
// drive the PDF FontDescriptor's Ascent/Descent. numberOfHMetrics
// equals numGlyphs so hmtx carries one longHorMetric per glyph and
// no leftSideBearings tail.
func buildHhea(numGlyphs int) []byte {
	buf := new(bytes.Buffer)
	write32(buf, 0x00010000) // version 1.0
	writeI16(buf, 800)       // ascender
	writeI16(buf, -200)      // descender
	writeI16(buf, 0)         // lineGap
	write16(buf, advance)    // advanceWidthMax
	writeI16(buf, 0)         // minLeftSideBearing
	writeI16(buf, 0)         // minRightSideBearing
	writeI16(buf, advance)   // xMaxExtent
	writeI16(buf, 1)         // caretSlopeRise
	writeI16(buf, 0)         // caretSlopeRun
	writeI16(buf, 0)         // caretOffset
	writeI16(buf, 0)         // reserved
	writeI16(buf, 0)
	writeI16(buf, 0)
	writeI16(buf, 0)
	writeI16(buf, 0)                // metricDataFormat
	write16(buf, uint16(numGlyphs)) // numberOfHMetrics
	return buf.Bytes()
}

// buildMaxp emits the 32-byte v1.0 maxp table (TrueType form with
// glyf/loca). Most maxima can be zero because every glyph in this
// fixture is empty (numberOfContours == 0).
func buildMaxp(numGlyphs int) []byte {
	buf := new(bytes.Buffer)
	write32(buf, 0x00010000) // version 1.0
	write16(buf, uint16(numGlyphs))
	write16(buf, 0) // maxPoints
	write16(buf, 0) // maxContours
	write16(buf, 0) // maxCompositePoints
	write16(buf, 0) // maxCompositeContours
	write16(buf, 2) // maxZones
	write16(buf, 0) // maxTwilightPoints
	write16(buf, 0) // maxStorage
	write16(buf, 0) // maxFunctionDefs
	write16(buf, 0) // maxInstructionDefs
	write16(buf, 0) // maxStackElements
	write16(buf, 0) // maxSizeOfInstructions
	write16(buf, 0) // maxComponentElements
	write16(buf, 0) // maxComponentDepth
	return buf.Bytes()
}

// buildCmap emits a cmap with a single subtable, format 4, encoding
// for Unicode BMP. Format 4 is segment-based: contiguous codepoint
// ranges map to glyph IDs via idDelta. The fixture's codepoints are
// scattered across CJK Unified Ideographs (U+4E00–U+9FFF) and a
// stray U+3002 in CJK Symbols, so each codepoint gets its own
// single-character segment. The mandatory final segment maps
// 0xFFFF → 0 (per spec).
func buildCmap(runes []rune) []byte {
	// Sort runes so segments are emitted in ascending order, which
	// format 4 requires.
	sorted := append([]rune(nil), runes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	type seg struct {
		end, start uint16
		delta      int16
	}
	var segs []seg
	// Each rune → its own segment. GID = position in original
	// `codepoints` slice + 1 (skip .notdef = GID 0).
	gidOf := make(map[rune]uint16, len(runes))
	for i, r := range runes {
		gidOf[r] = uint16(i + 1)
	}
	for _, r := range sorted {
		gid := gidOf[r]
		// idDelta is applied modulo 65536 to convert codepoint to GID.
		delta := int32(gid) - int32(r)
		// int16 wrap is fine — readers do (cp + delta) & 0xFFFF.
		segs = append(segs, seg{
			end:   uint16(r),
			start: uint16(r),
			delta: int16(delta),
		})
	}
	// Mandatory final segment 0xFFFF → 0.
	segs = append(segs, seg{end: 0xFFFF, start: 0xFFFF, delta: 1})

	segCount := uint16(len(segs))
	segCountX2 := segCount * 2
	// searchRange = 2 * 2^floor(log2(segCount))
	searchRange := uint16(1)
	for searchRange*2 <= segCount {
		searchRange *= 2
	}
	searchRange *= 2
	entrySelector := uint16(0)
	for (1 << entrySelector) < searchRange/2 {
		entrySelector++
	}
	rangeShift := segCountX2 - searchRange

	// Subtable.
	st := new(bytes.Buffer)
	write16(st, 4) // format
	write16(st, 0) // length (patched below)
	write16(st, 0) // language
	write16(st, segCountX2)
	write16(st, searchRange)
	write16(st, entrySelector)
	write16(st, rangeShift)
	for _, s := range segs {
		write16(st, s.end)
	}
	write16(st, 0) // reservedPad
	for _, s := range segs {
		write16(st, s.start)
	}
	for _, s := range segs {
		writeI16(st, s.delta)
	}
	for range segs {
		write16(st, 0) // idRangeOffset: 0 → use idDelta path
	}
	// No glyphIdArray needed.
	subtable := st.Bytes()
	binary.BigEndian.PutUint16(subtable[2:4], uint16(len(subtable)))

	// cmap header.
	buf := new(bytes.Buffer)
	write16(buf, 0) // version
	write16(buf, 1) // numTables
	// Encoding record: platform 3 (Windows), encoding 1 (UCS-2 BMP),
	// offset to subtable from start of cmap.
	write16(buf, 3)
	write16(buf, 1)
	write32(buf, 4+8) // header + one encoding record
	buf.Write(subtable)
	return buf.Bytes()
}

// buildName emits a minimal name table with two records: FontFamily
// (NameID 1) and PostScriptName (NameID 6). Both are UTF-16BE strings
// per platform 3 / encoding 1 (Windows Unicode BMP). Required so
// face.PostScriptName() returns a non-empty value — the embedder
// uses it as the PDF /BaseFont.
func buildName() []byte {
	type record struct {
		nameID uint16
		value  string
	}
	records := []record{
		{1, "Synthetic CJK"},
		{6, "SyntheticCJK"},
	}

	// Encode each value as UTF-16BE.
	type encoded struct {
		nameID uint16
		bytes  []byte
	}
	enc := make([]encoded, len(records))
	stringData := new(bytes.Buffer)
	for i, r := range records {
		var u16 bytes.Buffer
		for _, c := range r.value {
			write16(&u16, uint16(c))
		}
		enc[i] = encoded{r.nameID, u16.Bytes()}
		stringData.Write(u16.Bytes())
	}

	headerLen := 6 + len(records)*12

	buf := new(bytes.Buffer)
	write16(buf, 0)                    // format 0
	write16(buf, uint16(len(records))) // count
	write16(buf, uint16(headerLen))    // stringOffset
	offset := uint16(0)
	for _, e := range enc {
		write16(buf, 3)      // platformID = Microsoft
		write16(buf, 1)      // encodingID = Unicode BMP
		write16(buf, 0x0409) // languageID = en-US
		write16(buf, e.nameID)
		write16(buf, uint16(len(e.bytes))) // length
		write16(buf, offset)               // offset into string storage
		offset += uint16(len(e.bytes))
	}
	buf.Write(stringData.Bytes())
	return buf.Bytes()
}

// buildHmtx emits one longHorMetric per glyph. All glyphs share the
// same advance and zero LSB; rendering is irrelevant for this fixture
// so uniformity simplifies the build.
func buildHmtx(numGlyphs int) []byte {
	buf := new(bytes.Buffer)
	for range numGlyphs {
		write16(buf, advance) // advanceWidth
		writeI16(buf, 0)      // lsb
	}
	return buf.Bytes()
}

// buildGlyf emits one empty glyph per slot. Per OpenType §glyf, a
// simple glyph with numberOfContours == 0 may be encoded as the
// two-byte header alone — no bounding box or contour data follows.
// Empty glyphs render to nothing, which is fine: the integration
// test only round-trips text via the /ToUnicode CMap.
func buildGlyf(numGlyphs int) []byte {
	buf := new(bytes.Buffer)
	for range numGlyphs {
		writeI16(buf, 0) // numberOfContours = 0
	}
	return buf.Bytes()
}

// buildLoca emits glyph offsets in long-format (uint32) consistent
// with head.indexToLocFormat == 1. Each empty glyph is 2 bytes, so
// offset[i] = i*2.
func buildLoca(numGlyphs int) []byte {
	buf := new(bytes.Buffer)
	for i := 0; i <= numGlyphs; i++ {
		write32(buf, uint32(i*2))
	}
	return buf.Bytes()
}

// buildPost emits a 32-byte post table in format 3.0 — "no glyph
// names exported," which is the smallest valid post and matches what
// subset PDFs produce.
func buildPost() []byte {
	buf := new(bytes.Buffer)
	write32(buf, 0x00030000) // format 3.0
	write32(buf, 0)          // italicAngle (Fixed 16.16)
	writeI16(buf, -100)      // underlinePosition
	writeI16(buf, 50)        // underlineThickness
	write32(buf, 0)          // isFixedPitch
	write32(buf, 0)          // minMemType42
	write32(buf, 0)          // maxMemType42
	write32(buf, 0)          // minMemType1
	write32(buf, 0)          // maxMemType1
	return buf.Bytes()
}

// assembleTTF combines the tables into a TrueType binary. Tables in
// the directory must be sorted alphabetically by tag (per spec). Each
// table is 4-byte aligned in the file; the directory records absolute
// offsets and unpadded lengths.
func assembleTTF(tables map[string][]byte) []byte {
	// Sort tags.
	tags := make([]string, 0, len(tables))
	for t := range tables {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	numTables := len(tags)
	searchRange := uint16(1)
	for searchRange*2 <= uint16(numTables) {
		searchRange *= 2
	}
	searchRange *= 16
	entrySelector := uint16(0)
	for (1 << entrySelector) < searchRange/16 {
		entrySelector++
	}
	rangeShift := uint16(numTables)*16 - searchRange

	headerLen := 12 + numTables*16
	offset := headerLen

	// First pass: compute aligned offsets.
	type entry struct {
		tag    string
		offset uint32
		length uint32
		data   []byte
	}
	entries := make([]entry, numTables)
	for i, t := range tags {
		data := tables[t]
		entries[i] = entry{
			tag:    t,
			offset: uint32(offset),
			length: uint32(len(data)),
			data:   data,
		}
		padded := (len(data) + 3) &^ 3
		offset += padded
	}
	totalSize := offset

	// Build the binary.
	out := make([]byte, totalSize)
	// Offset table.
	binary.BigEndian.PutUint32(out[0:4], 0x00010000) // sfntVersion = TrueType
	binary.BigEndian.PutUint16(out[4:6], uint16(numTables))
	binary.BigEndian.PutUint16(out[6:8], searchRange)
	binary.BigEndian.PutUint16(out[8:10], entrySelector)
	binary.BigEndian.PutUint16(out[10:12], rangeShift)

	// Directory entries.
	for i, e := range entries {
		base := 12 + i*16
		copy(out[base:base+4], padTag(e.tag))
		binary.BigEndian.PutUint32(out[base+4:base+8], tableChecksum(e.data))
		binary.BigEndian.PutUint32(out[base+8:base+12], e.offset)
		binary.BigEndian.PutUint32(out[base+12:base+16], e.length)
	}

	// Payload.
	for _, e := range entries {
		copy(out[e.offset:e.offset+e.length], e.data)
	}
	return out
}

// padTag right-pads a 1-3 character tag to 4 bytes with spaces.
func padTag(tag string) []byte {
	out := []byte{' ', ' ', ' ', ' '}
	copy(out, tag)
	return out
}

// tableChecksum computes the OpenType per-table sum: the sum of all
// 32-bit big-endian words modulo 2^32. Padding bytes are treated as
// zero per spec.
func tableChecksum(data []byte) uint32 {
	var sum uint32
	for i := 0; i < len(data); i += 4 {
		var word uint32
		end := i + 4
		if end > len(data) {
			end = len(data)
		}
		for j := i; j < end; j++ {
			word |= uint32(data[j]) << (8 * (3 - (j - i)))
		}
		sum += word
	}
	return sum
}

// --- small byte-writer helpers ---

func write16(buf *bytes.Buffer, v uint16) {
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v))
}

func writeI16(buf *bytes.Buffer, v int16) {
	write16(buf, uint16(v))
}

func write32(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v >> 24))
	buf.WriteByte(byte(v >> 16))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v))
}

func write64(buf *bytes.Buffer, v uint64) {
	for i := 7; i >= 0; i-- {
		buf.WriteByte(byte(v >> (8 * i)))
	}
}
