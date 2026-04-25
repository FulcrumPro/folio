// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"testing"
)

// gposBuilder accumulates raw bytes. All synthetic GPOS tables in this
// file are assembled by hand so each test reads like a byte-level
// picture of the structure under test.
type gposBuilder struct {
	buf []byte
}

func (b *gposBuilder) u16(v uint16) {
	var tmp [2]byte
	binary.BigEndian.PutUint16(tmp[:], v)
	b.buf = append(b.buf, tmp[:]...)
}

func (b *gposBuilder) u32(v uint32) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], v)
	b.buf = append(b.buf, tmp[:]...)
}

func (b *gposBuilder) i16(v int16) { b.u16(uint16(v)) }

// patchU16 rewrites a previously reserved uint16 at position p.
func (b *gposBuilder) patchU16(p int, v uint16) {
	binary.BigEndian.PutUint16(b.buf[p:p+2], v)
}

// patchU32 rewrites a previously reserved uint32 at position p.
func (b *gposBuilder) patchU32(p int, v uint32) {
	binary.BigEndian.PutUint32(b.buf[p:p+4], v)
}

// pos returns the current write offset.
func (b *gposBuilder) pos() int { return len(b.buf) }

// wrapTTF builds a minimal single-font TTF wrapper containing a GPOS
// table whose body is the given bytes. The rest of the required tables
// are stubbed: we only need findTable to locate "GPOS".
func wrapTTF(gposBody []byte) []byte {
	// Header: sfntVersion(4), numTables(2), searchRange(2),
	// entrySelector(2), rangeShift(2). One table record: tag(4),
	// checkSum(4), offset(4), length(4). Then the body.
	const numTables = 1
	headerSize := 12 + numTables*16
	var out gposBuilder
	out.u32(0x00010000) // sfntVersion
	out.u16(numTables)
	out.u16(0) // searchRange
	out.u16(0) // entrySelector
	out.u16(0) // rangeShift
	// Table record for GPOS.
	out.buf = append(out.buf, []byte("GPOS")...)
	out.u32(0) // checkSum
	out.u32(uint32(headerSize))
	out.u32(uint32(len(gposBody)))
	out.buf = append(out.buf, gposBody...)
	return out.buf
}

// buildGPOSHeader produces a GPOS header that points at a single "DFLT"
// script with one LangSys referencing one feature, one feature, and one
// lookup. The caller supplies the feature tag and the raw lookup
// subtable bytes (lookupBody). A lookup-type code identifies the
// LookupType field. extensionType controls whether the lookup is wrapped
// in a LookupType 9 extension indirection; pass 0 for no wrapping.
//
// The return value is the full GPOS table bytes.
func buildGPOSHeader(featureTag string, lookupType uint16, lookupBody []byte, extensionType uint16) []byte {
	var b gposBuilder

	// Header: version(4), scriptListOffset(2), featureListOffset(2),
	// lookupListOffset(2). Offsets are from the start of the GPOS
	// table, and we will patch them once we know their positions.
	b.u32(0x00010000)
	scriptListPos := b.pos()
	b.u16(0)
	featureListPos := b.pos()
	b.u16(0)
	lookupListPos := b.pos()
	b.u16(0)

	// ScriptList.
	scriptListOff := b.pos()
	b.patchU16(scriptListPos, uint16(scriptListOff))
	b.u16(1) // scriptCount
	b.buf = append(b.buf, []byte("DFLT")...)
	scriptRecordOffPos := b.pos()
	b.u16(0) // scriptOffset placeholder (from ScriptList start)

	// Script table.
	scriptTableOff := b.pos()
	b.patchU16(scriptRecordOffPos, uint16(scriptTableOff-scriptListOff))
	defaultLangSysPos := b.pos()
	b.u16(0) // defaultLangSysOffset placeholder
	b.u16(0) // langSysCount

	// Default LangSys table.
	defaultLangSysOff := b.pos()
	b.patchU16(defaultLangSysPos, uint16(defaultLangSysOff-scriptTableOff))
	b.u16(0) // lookupOrder (reserved)
	b.u16(0xFFFF)
	b.u16(1) // featureIndexCount
	b.u16(0) // referenced feature index 0

	// FeatureList.
	featureListOff := b.pos()
	b.patchU16(featureListPos, uint16(featureListOff))
	b.u16(1) // featureCount
	b.buf = append(b.buf, []byte(featureTag)...)
	featureRecordOffPos := b.pos()
	b.u16(0) // featureOffset placeholder

	// Feature table.
	featureTableOff := b.pos()
	b.patchU16(featureRecordOffPos, uint16(featureTableOff-featureListOff))
	b.u16(0) // featureParams
	b.u16(1) // lookupIndexCount
	b.u16(0) // lookupListIndices[0]

	// LookupList.
	lookupListOff := b.pos()
	b.patchU16(lookupListPos, uint16(lookupListOff))
	b.u16(1) // lookupCount
	lookupRecordOffPos := b.pos()
	b.u16(0) // lookup offset placeholder

	// Lookup table.
	lookupTableOff := b.pos()
	b.patchU16(lookupRecordOffPos, uint16(lookupTableOff-lookupListOff))
	if extensionType != 0 {
		b.u16(9) // lookupType = extension
	} else {
		b.u16(lookupType)
	}
	b.u16(0) // lookupFlag
	b.u16(1) // subTableCount
	subOffPos := b.pos()
	b.u16(0) // subtableOffset placeholder (from lookup start)

	if extensionType != 0 {
		// Extension subtable: format(2), extensionLookupType(2),
		// extensionOffset(4, from start of this subtable).
		extSubOff := b.pos()
		b.patchU16(subOffPos, uint16(extSubOff-lookupTableOff))
		b.u16(1) // posFormat
		b.u16(extensionType)
		extOffPos := b.pos()
		b.u32(0) // extensionOffset placeholder (from extSubOff)
		realSubOff := b.pos()
		b.patchU32(extOffPos, uint32(realSubOff-extSubOff))
		b.buf = append(b.buf, lookupBody...)
	} else {
		subOff := b.pos()
		b.patchU16(subOffPos, uint16(subOff-lookupTableOff))
		b.buf = append(b.buf, lookupBody...)
	}

	return b.buf
}

// buildCoverageFormat1 produces a Coverage Format 1 table listing gids
// in order. The caller must ensure gids are strictly ascending.
func buildCoverageFormat1(gids ...uint16) []byte {
	var b gposBuilder
	b.u16(1)
	b.u16(uint16(len(gids)))
	for _, g := range gids {
		b.u16(g)
	}
	return b.buf
}

// buildClassDefFormat2 produces a ClassDef Format 2 from range records
// each describing { startGID, endGID, class }.
type classRange struct {
	start, end, class uint16
}

func buildClassDefFormat2(ranges ...classRange) []byte {
	var b gposBuilder
	b.u16(2)
	b.u16(uint16(len(ranges)))
	for _, r := range ranges {
		b.u16(r.start)
		b.u16(r.end)
		b.u16(r.class)
	}
	return b.buf
}

// TestValueRecordSize locks the size-counting helper against every
// single-bit and a couple of combinations. Pair kerning only uses bit 2
// but the helper must skip past all preceding fields correctly.
func TestValueRecordSize(t *testing.T) {
	cases := []struct {
		vf   uint16
		want int
	}{
		{0x0000, 0},
		{0x0001, 2}, // XPlacement
		{0x0002, 2}, // YPlacement
		{0x0004, 2}, // XAdvance
		{0x0005, 4}, // XPlacement + XAdvance
		{0x000F, 8}, // all four placements/advances
		{0x00FF, 16},
	}
	for _, c := range cases {
		if got := valueRecordSize(c.vf); got != c.want {
			t.Errorf("valueRecordSize(%#x) = %d, want %d", c.vf, got, c.want)
		}
	}
}

// TestValueRecordXAdvance verifies the XAdvance reader skips preceding
// XPlacement/YPlacement fields so that a valueFormat of 0x05 lands at
// the second int16 rather than the first.
func TestValueRecordXAdvance(t *testing.T) {
	// ValueRecord with XPlacement=7, XAdvance=-50. valueFormat = 0x05.
	var b gposBuilder
	b.i16(7)
	b.i16(-50)
	got := valueRecordXAdvance(b.buf, 0, 0x0005)
	if got != -50 {
		t.Errorf("XAdvance with XPlacement present = %d, want -50", got)
	}

	// Bit 2 clear: must return 0 regardless of buffer contents.
	if v := valueRecordXAdvance(b.buf, 0, 0x0003); v != 0 {
		t.Errorf("XAdvance with bit 2 clear = %d, want 0", v)
	}
}

// pairPosFormat1Body assembles a PairPosFormat1 subtable with a single
// coverage entry (leftGID) and a single PairSet containing a single
// pair (leftGID, rightGID) with the given XAdvance. valueFormat1 is
// caller-controlled to let tests exercise ValueFormat masking; the
// valueRecord is always laid out matching the declared format but only
// the XAdvance field carries the test value.
func pairPosFormat1Body(leftGID, rightGID uint16, xAdvance int16, valueFormat1 uint16) []byte {
	var b gposBuilder
	b.u16(1) // posFormat
	covOffPos := b.pos()
	b.u16(0) // coverageOffset placeholder
	b.u16(valueFormat1)
	b.u16(0) // valueFormat2
	b.u16(1) // pairSetCount
	setOffPos := b.pos()
	b.u16(0) // pairSetOffset[0] placeholder

	// Coverage table at current position.
	covOff := b.pos()
	b.patchU16(covOffPos, uint16(covOff))
	b.buf = append(b.buf, buildCoverageFormat1(leftGID)...)

	// PairSet table.
	setOff := b.pos()
	b.patchU16(setOffPos, uint16(setOff))
	b.u16(1) // pairValueCount
	b.u16(rightGID)
	// Write a valueRecord1 matching valueFormat1.
	// Field order per the spec: XPlacement, YPlacement, XAdvance.
	if valueFormat1&0x0001 != 0 {
		b.i16(0) // XPlacement filler
	}
	if valueFormat1&0x0002 != 0 {
		b.i16(0) // YPlacement filler
	}
	if valueFormat1&0x0004 != 0 {
		b.i16(xAdvance)
	}
	return b.buf
}

// TestParseGPOSPairPosFormat1 covers the happy path of LookupType 2
// Format 1: one pair, one adjustment, plus a miss lookup.
func TestParseGPOSPairPosFormat1(t *testing.T) {
	body := pairPosFormat1Body(10, 20, -50, 0x0004)
	gpos := buildGPOSHeader("kern", 2, body, 0)
	font := wrapTTF(gpos)

	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	if got := g.PairAdjust(10, 20); got != -50 {
		t.Errorf("PairAdjust(10,20) = %d, want -50", got)
	}
	if got := g.PairAdjust(10, 21); got != 0 {
		t.Errorf("PairAdjust(10,21) = %d, want 0", got)
	}
	if got := g.PairAdjust(11, 20); got != 0 {
		t.Errorf("PairAdjust(11,20) = %d, want 0", got)
	}
}

// TestParseGPOSPairPosValueFormatMasking verifies that a ValueRecord
// carrying both XPlacement and XAdvance still yields the correct
// XAdvance, i.e. the parser walks past the XPlacement slot.
func TestParseGPOSPairPosValueFormatMasking(t *testing.T) {
	// valueFormat1 = 0x05 -> XPlacement + XAdvance.
	body := pairPosFormat1Body(5, 6, -42, 0x0005)
	gpos := buildGPOSHeader("kern", 2, body, 0)
	font := wrapTTF(gpos)

	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	if got := g.PairAdjust(5, 6); got != -42 {
		t.Errorf("PairAdjust(5,6) with XPlacement+XAdvance = %d, want -42", got)
	}
}

// TestParseGPOSPairPosFormat2 exercises class-based pair positioning:
// two left-classes x two right-classes with four distinct adjustments.
func TestParseGPOSPairPosFormat2(t *testing.T) {
	// Left-class map: GID 10 -> class 1, GID 11 -> class 2.
	// Right-class map: GID 20 -> class 1, GID 21 -> class 2.
	// Coverage picks up {10, 11} as eligible left glyphs.
	//
	// Expected adjustments (class1, class2) -> value:
	//   (1,1) = -10   -> (10,20)
	//   (1,2) = -20   -> (10,21)
	//   (2,1) = -30   -> (11,20)
	//   (2,2) = -40   -> (11,21)
	var b gposBuilder
	b.u16(2) // posFormat
	covOffPos := b.pos()
	b.u16(0) // coverage
	b.u16(0x0004)
	b.u16(0)
	cd1OffPos := b.pos()
	b.u16(0) // classDef1Offset
	cd2OffPos := b.pos()
	b.u16(0) // classDef2Offset
	b.u16(3) // class1Count (includes class 0)
	b.u16(3) // class2Count (includes class 0)

	// class1Records[0]: class 0 row (3 class2 records, all zeros).
	// class1Records[1]: class 1 row.
	// class1Records[2]: class 2 row.
	// Each class2 record here is one int16 (XAdvance).
	b.i16(0) // (0,0)
	b.i16(0) // (0,1)
	b.i16(0) // (0,2)

	b.i16(0)   // (1,0)
	b.i16(-10) // (1,1) -> (10,20)
	b.i16(-20) // (1,2) -> (10,21)

	b.i16(0)   // (2,0)
	b.i16(-30) // (2,1) -> (11,20)
	b.i16(-40) // (2,2) -> (11,21)

	covOff := b.pos()
	b.patchU16(covOffPos, uint16(covOff))
	b.buf = append(b.buf, buildCoverageFormat1(10, 11)...)

	cd1Off := b.pos()
	b.patchU16(cd1OffPos, uint16(cd1Off))
	b.buf = append(b.buf, buildClassDefFormat2(
		classRange{start: 10, end: 10, class: 1},
		classRange{start: 11, end: 11, class: 2},
	)...)

	cd2Off := b.pos()
	b.patchU16(cd2OffPos, uint16(cd2Off))
	b.buf = append(b.buf, buildClassDefFormat2(
		classRange{start: 20, end: 20, class: 1},
		classRange{start: 21, end: 21, class: 2},
	)...)

	gpos := buildGPOSHeader("kern", 2, b.buf, 0)
	font := wrapTTF(gpos)

	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	cases := []struct {
		l, r uint16
		want int16
	}{
		{10, 20, -10},
		{10, 21, -20},
		{11, 20, -30},
		{11, 21, -40},
		{10, 99, 0},
		{99, 20, 0},
	}
	for _, c := range cases {
		if got := g.PairAdjust(c.l, c.r); got != c.want {
			t.Errorf("PairAdjust(%d,%d) = %d, want %d", c.l, c.r, got, c.want)
		}
	}
}

// markBasePosBody assembles a MarkBasePosFormat1 subtable with one mark
// coverage entry (markGID), one base coverage entry (baseGID), one mark
// class, and an anchor written in the given Anchor format. The mark and
// base anchors use independently specified (x,y) pairs.
func markBasePosBody(markGID, baseGID uint16, markX, markY, baseX, baseY int16, anchorFormat uint16) []byte {
	var b gposBuilder
	b.u16(1) // posFormat
	markCovOffPos := b.pos()
	b.u16(0) // markCoverageOffset
	baseCovOffPos := b.pos()
	b.u16(0) // baseCoverageOffset
	b.u16(1) // markClassCount
	markArrayOffPos := b.pos()
	b.u16(0) // markArrayOffset
	baseArrayOffPos := b.pos()
	b.u16(0) // baseArrayOffset

	markCovOff := b.pos()
	b.patchU16(markCovOffPos, uint16(markCovOff))
	b.buf = append(b.buf, buildCoverageFormat1(markGID)...)

	baseCovOff := b.pos()
	b.patchU16(baseCovOffPos, uint16(baseCovOff))
	b.buf = append(b.buf, buildCoverageFormat1(baseGID)...)

	markArrayOff := b.pos()
	b.patchU16(markArrayOffPos, uint16(markArrayOff))
	b.u16(1) // markCount
	b.u16(0) // markClass
	markAnchorOffPos := b.pos()
	b.u16(0) // markAnchorOffset (from markArrayOff)

	baseArrayOff := b.pos()
	b.patchU16(baseArrayOffPos, uint16(baseArrayOff))
	b.u16(1) // baseCount
	baseAnchorOffPos := b.pos()
	b.u16(0) // baseAnchorOffset (from baseArrayOff)

	// Mark anchor.
	markAnchorOff := b.pos()
	b.patchU16(markAnchorOffPos, uint16(markAnchorOff-markArrayOff))
	b.u16(anchorFormat)
	b.i16(markX)
	b.i16(markY)
	switch anchorFormat {
	case 2:
		b.u16(0) // anchorPoint index (ignored)
	case 3:
		b.u16(0) // xDeviceOffset (ignored)
		b.u16(0) // yDeviceOffset (ignored)
	}

	// Base anchor.
	baseAnchorOff := b.pos()
	b.patchU16(baseAnchorOffPos, uint16(baseAnchorOff-baseArrayOff))
	b.u16(anchorFormat)
	b.i16(baseX)
	b.i16(baseY)
	switch anchorFormat {
	case 2:
		b.u16(0)
	case 3:
		b.u16(0)
		b.u16(0)
	}

	return b.buf
}

// TestParseGPOSMarkBasePosFormat1 checks that a mark with its anchor at
// (200,300) attached to a base with anchor (500,800) yields an offset of
// (300, 500) — i.e. base.X - mark.X, base.Y - mark.Y.
func TestParseGPOSMarkBasePosFormat1(t *testing.T) {
	body := markBasePosBody(50, 100, 200, 300, 500, 800, 1)
	gpos := buildGPOSHeader("mark", 4, body, 0)
	font := wrapTTF(gpos)

	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	dx, dy, ok := g.MarkOffset(100, 50, GPOSMark)
	if !ok {
		t.Fatal("MarkOffset returned ok=false, want true")
	}
	if dx != 300 || dy != 500 {
		t.Errorf("MarkOffset = (%d, %d), want (300, 500)", dx, dy)
	}
	if _, _, okMiss := g.MarkOffset(999, 50, GPOSMark); okMiss {
		t.Error("MarkOffset for unknown base should return ok=false")
	}
	if _, _, okMiss := g.MarkOffset(100, 999, GPOSMark); okMiss {
		t.Error("MarkOffset for unknown mark should return ok=false")
	}
}

// TestParseGPOSAnchorFormats2And3 locks down that anchor formats 2 and 3
// yield the same x,y as format 1 for the same base values. The extra
// anchor-point index and Device offsets must be read past without
// polluting the extracted coordinates.
func TestParseGPOSAnchorFormats2And3(t *testing.T) {
	for _, fmt := range []uint16{1, 2, 3} {
		body := markBasePosBody(50, 100, 10, 20, 30, 40, fmt)
		gpos := buildGPOSHeader("mark", 4, body, 0)
		font := wrapTTF(gpos)

		g := ParseGPOS(font)
		if g == nil {
			t.Fatalf("format %d: ParseGPOS returned nil", fmt)
		}
		dx, dy, ok := g.MarkOffset(100, 50, GPOSMark)
		if !ok {
			t.Fatalf("format %d: MarkOffset ok=false", fmt)
		}
		if dx != 20 || dy != 20 {
			t.Errorf("format %d: MarkOffset = (%d, %d), want (20, 20)", fmt, dx, dy)
		}
	}
}

// TestParseGPOSMarkMarkPosFormat1 checks the LookupType 6 parser: a
// mark1 whose anchor sits at (100, 400) attaching to a mark2 whose
// anchor sits at (150, 900) must yield an offset of (50, 500), i.e.
// mark2.X - mark1.X, mark2.Y - mark1.Y. The byte layout of a
// MarkMarkPosFormat1 subtable is identical to MarkBasePosFormat1, so
// markBasePosBody is reused here with the mark1/mark2 glyph IDs
// standing in for mark/base.
func TestParseGPOSMarkMarkPosFormat1(t *testing.T) {
	const (
		mark1GID uint16 = 61 // attaching mark (e.g. shadda on top)
		mark2GID uint16 = 60 // underlying mark (e.g. fatha)
	)
	body := markBasePosBody(mark1GID, mark2GID, 100, 400, 150, 900, 1)
	gpos := buildGPOSHeader("mkmk", 6, body, 0)
	font := wrapTTF(gpos)

	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	dx, dy, ok := g.MarkMarkOffset(mark1GID, mark2GID, GPOSMkmk)
	if !ok {
		t.Fatal("MarkMarkOffset returned ok=false, want true")
	}
	if dx != 50 || dy != 500 {
		t.Errorf("MarkMarkOffset = (%d, %d), want (50, 500)", dx, dy)
	}
	// The reverse pair must miss: mark2 is not in the mark1 array.
	if _, _, okMiss := g.MarkMarkOffset(mark2GID, mark1GID, GPOSMkmk); okMiss {
		t.Error("MarkMarkOffset with swapped operands should return ok=false")
	}
	if _, _, okMiss := g.MarkMarkOffset(999, mark2GID, GPOSMkmk); okMiss {
		t.Error("MarkMarkOffset for unknown mark1 should return ok=false")
	}
	if _, _, okMiss := g.MarkMarkOffset(mark1GID, 999, GPOSMkmk); okMiss {
		t.Error("MarkMarkOffset for unknown mark2 should return ok=false")
	}
	// Mark-to-base storage must remain independent: mkmk data must not
	// bleed into the mark feature.
	if _, _, okMiss := g.MarkOffset(mark2GID, mark1GID, GPOSMark); okMiss {
		t.Error("mkmk data must not populate the mark feature's MarkOffset path")
	}
}

// TestParseGPOSMarkMarkExtensionWrap verifies that LookupType 6 is
// followed through a LookupType 9 extension subtable, matching the
// treatment of LookupType 2 and 4.
func TestParseGPOSMarkMarkExtensionWrap(t *testing.T) {
	body := markBasePosBody(61, 60, 10, 40, 30, 90, 1)
	gpos := buildGPOSHeader("mkmk", 6, body, 6)
	font := wrapTTF(gpos)

	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	dx, dy, ok := g.MarkMarkOffset(61, 60, GPOSMkmk)
	if !ok {
		t.Fatal("MarkMarkOffset through extension returned ok=false")
	}
	if dx != 20 || dy != 50 {
		t.Errorf("MarkMarkOffset through extension = (%d, %d), want (20, 50)", dx, dy)
	}
}

// TestMarkMarkOffsetNilReceiver covers the nil-safety contract; the
// draw pipeline relies on a zero-value return when GPOS parsing
// yielded nothing.
func TestMarkMarkOffsetNilReceiver(t *testing.T) {
	var g *GPOSAdjustments
	if _, _, ok := g.MarkMarkOffset(1, 2, GPOSMkmk); ok {
		t.Error("MarkMarkOffset on nil receiver must return ok=false")
	}
}

// markMarkPosBodyMultiClass assembles a MarkMarkPosFormat1 subtable
// with markClassCount=2, two mark1 glyphs in different classes, and
// one mark2 that carries two anchors (one per class). The returned
// body exercises the class-indexed anchor lookup which is invisible
// under the single-class test.
//
// Layout reminder (same as MarkBasePosFormat1):
//
//	posFormat(2) mark1CovOff(2) mark2CovOff(2) markClassCount(2)
//	mark1ArrayOff(2) mark2ArrayOff(2)
//	... Mark1Coverage, Mark2Coverage, Mark1Array, Mark2Array, Anchors
func markMarkPosBodyMultiClass() []byte {
	const (
		mark1GIDA uint16 = 70 // class 0 attaching mark
		mark1GIDB uint16 = 71 // class 1 attaching mark
		mark2GID  uint16 = 72 // underlying mark with two anchors
	)
	_ = mark1GIDA
	_ = mark1GIDB
	_ = mark2GID

	var b gposBuilder
	b.u16(1) // posFormat
	mark1CovOffPos := b.pos()
	b.u16(0) // mark1CoverageOffset
	mark2CovOffPos := b.pos()
	b.u16(0) // mark2CoverageOffset
	b.u16(2) // markClassCount
	mark1ArrayOffPos := b.pos()
	b.u16(0)
	mark2ArrayOffPos := b.pos()
	b.u16(0)

	// Mark1 coverage: {70, 71}.
	mark1CovOff := b.pos()
	b.patchU16(mark1CovOffPos, uint16(mark1CovOff))
	b.buf = append(b.buf, buildCoverageFormat1(70, 71)...)

	// Mark2 coverage: {72}.
	mark2CovOff := b.pos()
	b.patchU16(mark2CovOffPos, uint16(mark2CovOff))
	b.buf = append(b.buf, buildCoverageFormat1(72)...)

	// Mark1Array: two entries, (class 0, anchor offset A), (class 1, anchor offset B).
	mark1ArrayOff := b.pos()
	b.patchU16(mark1ArrayOffPos, uint16(mark1ArrayOff))
	b.u16(2)                // markCount
	b.u16(0)                // mark1GIDA class
	mark1AOffPos := b.pos() // anchor A offset
	b.u16(0)
	b.u16(1)                // mark1GIDB class
	mark1BOffPos := b.pos() // anchor B offset
	b.u16(0)

	// Mark2Array: one entry with 2 anchor offsets (one per class).
	mark2ArrayOff := b.pos()
	b.patchU16(mark2ArrayOffPos, uint16(mark2ArrayOff))
	b.u16(1) // mark2Count
	mark2AOffPos := b.pos()
	b.u16(0) // mark2 class-0 anchor offset
	mark2BOffPos := b.pos()
	b.u16(0) // mark2 class-1 anchor offset

	// Anchors. Mark1A = (10, 20), Mark1B = (30, 40);
	// Mark2 class-0 = (100, 200), Mark2 class-1 = (300, 400).
	// Expected offsets:
	//   (mark1A, mark2) class 0 → (100-10, 200-20) = (90, 180)
	//   (mark1B, mark2) class 1 → (300-30, 400-40) = (270, 360)
	mark1AOff := b.pos()
	b.patchU16(mark1AOffPos, uint16(mark1AOff-mark1ArrayOff))
	b.u16(1) // Anchor format 1
	b.i16(10)
	b.i16(20)

	mark1BOff := b.pos()
	b.patchU16(mark1BOffPos, uint16(mark1BOff-mark1ArrayOff))
	b.u16(1)
	b.i16(30)
	b.i16(40)

	mark2AOff := b.pos()
	b.patchU16(mark2AOffPos, uint16(mark2AOff-mark2ArrayOff))
	b.u16(1)
	b.i16(100)
	b.i16(200)

	mark2BOff := b.pos()
	b.patchU16(mark2BOffPos, uint16(mark2BOff-mark2ArrayOff))
	b.u16(1)
	b.i16(300)
	b.i16(400)

	return b.buf
}

// TestParseGPOSMarkMarkMultiClass locks down the class-indexed anchor
// lookup: mark1A (class 0) must resolve against mark2's class-0 anchor,
// and mark1B (class 1) against mark2's class-1 anchor. A regression
// that hard-coded class 0 would be caught here.
func TestParseGPOSMarkMarkMultiClass(t *testing.T) {
	body := markMarkPosBodyMultiClass()
	gpos := buildGPOSHeader("mkmk", 6, body, 0)
	font := wrapTTF(gpos)

	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}

	dx, dy, ok := g.MarkMarkOffset(70, 72, GPOSMkmk)
	if !ok {
		t.Fatal("MarkMarkOffset(70,72) ok=false")
	}
	if dx != 90 || dy != 180 {
		t.Errorf("MarkMarkOffset(70,72) class 0 = (%d,%d), want (90,180)", dx, dy)
	}

	dx, dy, ok = g.MarkMarkOffset(71, 72, GPOSMkmk)
	if !ok {
		t.Fatal("MarkMarkOffset(71,72) ok=false")
	}
	if dx != 270 || dy != 360 {
		t.Errorf("MarkMarkOffset(71,72) class 1 = (%d,%d), want (270,360)", dx, dy)
	}
}

// TestParseGPOSMarkMarkClassOverflow covers the defensive branch in
// anchorOffset: a mark1 whose Class is out of range for the mark2's
// anchor slice returns ok=false rather than panicking.
func TestParseGPOSMarkMarkClassOverflow(t *testing.T) {
	// Build a mkmk body with markClassCount=1 but force the mark1
	// record to claim class 5. The parser stores the class verbatim;
	// lookup must refuse to index past the mark2 anchor slice.
	var b gposBuilder
	b.u16(1) // posFormat
	mark1CovOffPos := b.pos()
	b.u16(0)
	mark2CovOffPos := b.pos()
	b.u16(0)
	b.u16(1) // markClassCount
	mark1ArrayOffPos := b.pos()
	b.u16(0)
	mark2ArrayOffPos := b.pos()
	b.u16(0)

	mark1CovOff := b.pos()
	b.patchU16(mark1CovOffPos, uint16(mark1CovOff))
	b.buf = append(b.buf, buildCoverageFormat1(80)...)
	mark2CovOff := b.pos()
	b.patchU16(mark2CovOffPos, uint16(mark2CovOff))
	b.buf = append(b.buf, buildCoverageFormat1(81)...)

	mark1ArrayOff := b.pos()
	b.patchU16(mark1ArrayOffPos, uint16(mark1ArrayOff))
	b.u16(1)
	b.u16(5) // out-of-range mark class
	mark1AOffPos := b.pos()
	b.u16(0)

	mark2ArrayOff := b.pos()
	b.patchU16(mark2ArrayOffPos, uint16(mark2ArrayOff))
	b.u16(1)
	mark2AOffPos := b.pos()
	b.u16(0)

	mark1AOff := b.pos()
	b.patchU16(mark1AOffPos, uint16(mark1AOff-mark1ArrayOff))
	b.u16(1)
	b.i16(0)
	b.i16(0)

	mark2AOff := b.pos()
	b.patchU16(mark2AOffPos, uint16(mark2AOff-mark2ArrayOff))
	b.u16(1)
	b.i16(10)
	b.i16(20)

	gpos := buildGPOSHeader("mkmk", 6, b.buf, 0)
	font := wrapTTF(gpos)
	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	if _, _, ok := g.MarkMarkOffset(80, 81, GPOSMkmk); ok {
		t.Error("MarkMarkOffset with out-of-range class must return ok=false")
	}
}

// buildGPOSHeaderDualFeature builds a GPOS table exposing two features
// (tag1 → lookupType1 body1, tag2 → lookupType2 body2) both wired
// through the default LangSys of a "DFLT" script. Exists to exercise
// the mark + mkmk coexistence path.
func buildGPOSHeaderDualFeature(tag1 string, lt1 uint16, body1 []byte, tag2 string, lt2 uint16, body2 []byte) []byte {
	var b gposBuilder
	b.u32(0x00010000)
	scriptListPos := b.pos()
	b.u16(0)
	featureListPos := b.pos()
	b.u16(0)
	lookupListPos := b.pos()
	b.u16(0)

	// ScriptList.
	scriptListOff := b.pos()
	b.patchU16(scriptListPos, uint16(scriptListOff))
	b.u16(1) // scriptCount
	b.buf = append(b.buf, []byte("DFLT")...)
	scriptRecordOffPos := b.pos()
	b.u16(0)

	scriptTableOff := b.pos()
	b.patchU16(scriptRecordOffPos, uint16(scriptTableOff-scriptListOff))
	defaultLangSysPos := b.pos()
	b.u16(0)
	b.u16(0) // langSysCount

	defaultLangSysOff := b.pos()
	b.patchU16(defaultLangSysPos, uint16(defaultLangSysOff-scriptTableOff))
	b.u16(0)      // lookupOrder
	b.u16(0xFFFF) // requiredFeatureIndex
	b.u16(2)      // featureIndexCount
	b.u16(0)      // feature 0
	b.u16(1)      // feature 1

	// FeatureList.
	featureListOff := b.pos()
	b.patchU16(featureListPos, uint16(featureListOff))
	b.u16(2) // featureCount
	b.buf = append(b.buf, []byte(tag1)...)
	f1OffPos := b.pos()
	b.u16(0)
	b.buf = append(b.buf, []byte(tag2)...)
	f2OffPos := b.pos()
	b.u16(0)

	feat1Off := b.pos()
	b.patchU16(f1OffPos, uint16(feat1Off-featureListOff))
	b.u16(0) // featureParams
	b.u16(1) // lookupIndexCount
	b.u16(0) // lookup 0

	feat2Off := b.pos()
	b.patchU16(f2OffPos, uint16(feat2Off-featureListOff))
	b.u16(0)
	b.u16(1)
	b.u16(1) // lookup 1

	// LookupList.
	lookupListOff := b.pos()
	b.patchU16(lookupListPos, uint16(lookupListOff))
	b.u16(2) // lookupCount
	l1OffPos := b.pos()
	b.u16(0)
	l2OffPos := b.pos()
	b.u16(0)

	// Lookup 0 (feature 1).
	lookup1Off := b.pos()
	b.patchU16(l1OffPos, uint16(lookup1Off-lookupListOff))
	b.u16(lt1)
	b.u16(0) // lookupFlag
	b.u16(1) // subTableCount
	sub1OffPos := b.pos()
	b.u16(0)

	sub1Off := b.pos()
	b.patchU16(sub1OffPos, uint16(sub1Off-lookup1Off))
	b.buf = append(b.buf, body1...)

	// Lookup 1 (feature 2).
	lookup2Off := b.pos()
	b.patchU16(l2OffPos, uint16(lookup2Off-lookupListOff))
	b.u16(lt2)
	b.u16(0)
	b.u16(1)
	sub2OffPos := b.pos()
	b.u16(0)

	sub2Off := b.pos()
	b.patchU16(sub2OffPos, uint16(sub2Off-lookup2Off))
	b.buf = append(b.buf, body2...)

	return b.buf
}

// TestParseGPOSMarkAndMkmkCoexist builds a GPOS blob that carries both
// a mark (LookupType 4) subtable and an mkmk (LookupType 6) subtable.
// Each must populate its own storage slot; neither must bleed into
// the other's maps. Catches any accidental shared accumulator.
func TestParseGPOSMarkAndMkmkCoexist(t *testing.T) {
	markBody := markBasePosBody(50, 100, 200, 300, 500, 800, 1)
	mkmkBody := markBasePosBody(61, 60, 100, 400, 150, 950, 1)

	gpos := buildGPOSHeaderDualFeature("mark", 4, markBody, "mkmk", 6, mkmkBody)
	font := wrapTTF(gpos)
	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}

	// Mark feature resolves.
	dx, dy, ok := g.MarkOffset(100, 50, GPOSMark)
	if !ok || dx != 300 || dy != 500 {
		t.Errorf("MarkOffset = (%d,%d,%v), want (300,500,true)", dx, dy, ok)
	}
	// Mkmk feature resolves independently.
	dx, dy, ok = g.MarkMarkOffset(61, 60, GPOSMkmk)
	if !ok || dx != 50 || dy != 550 {
		t.Errorf("MarkMarkOffset = (%d,%d,%v), want (50,550,true)", dx, dy, ok)
	}
	// Mark data must not leak into mkmk storage and vice versa.
	if len(g.MarkMarks[GPOSMark]) != 0 {
		t.Errorf("MarkMarks[GPOSMark] should be empty, got %v", g.MarkMarks[GPOSMark])
	}
	if len(g.Mark2Bases[GPOSMark]) != 0 {
		t.Errorf("Mark2Bases[GPOSMark] should be empty, got %v", g.Mark2Bases[GPOSMark])
	}
	if len(g.Marks[GPOSMkmk]) != 0 {
		t.Errorf("Marks[GPOSMkmk] should be empty, got %v", g.Marks[GPOSMkmk])
	}
	if len(g.Bases[GPOSMkmk]) != 0 {
		t.Errorf("Bases[GPOSMkmk] should be empty, got %v", g.Bases[GPOSMkmk])
	}
}

// TestParseGPOSMkmkOnlyReturnsNonNil pins the nil-guard at the bottom
// of ParseGPOS: a font whose GPOS table carries only an mkmk feature
// (no kern, no mark) must still produce a non-nil result. A regression
// that forgot to include MarkMarks/Mark2Bases in the guard would make
// mkmk-only fonts silently lose their positioning data.
func TestParseGPOSMkmkOnlyReturnsNonNil(t *testing.T) {
	body := markBasePosBody(61, 60, 100, 400, 150, 950, 1)
	gpos := buildGPOSHeader("mkmk", 6, body, 0)
	g := ParseGPOS(wrapTTF(gpos))
	if g == nil {
		t.Fatal("ParseGPOS with only mkmk feature returned nil")
	}
	if len(g.Pairs) != 0 || len(g.Marks) != 0 || len(g.Bases) != 0 {
		t.Errorf("mkmk-only font populated non-mkmk maps: pairs=%d marks=%d bases=%d",
			len(g.Pairs), len(g.Marks), len(g.Bases))
	}
	if len(g.MarkMarks[GPOSMkmk]) == 0 || len(g.Mark2Bases[GPOSMkmk]) == 0 {
		t.Error("mkmk-only font did not populate MarkMarks/Mark2Bases")
	}
}

// TestParseGPOSExtensionWrap wraps a PairPosFormat1 subtable inside a
// LookupType 9 Extension and verifies it still resolves.
func TestParseGPOSExtensionWrap(t *testing.T) {
	body := pairPosFormat1Body(42, 43, -25, 0x0004)
	gpos := buildGPOSHeader("kern", 2, body, 2)
	font := wrapTTF(gpos)

	g := ParseGPOS(font)
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	if got := g.PairAdjust(42, 43); got != -25 {
		t.Errorf("PairAdjust through extension = %d, want -25", got)
	}
}

// TestParseGPOSReturnsNilWithoutTable sanity-checks the nil return path
// for fonts that don't carry a GPOS table.
func TestParseGPOSReturnsNilWithoutTable(t *testing.T) {
	if ParseGPOS(nil) != nil {
		t.Error("ParseGPOS(nil) should return nil")
	}
	if ParseGPOS([]byte{0}) != nil {
		t.Error("ParseGPOS short buffer should return nil")
	}
}

// TestFaceKernPrefersGPOS verifies that a Face whose backing font has
// both a GPOS kern pair and a legacy kern pair for the same (l,r) pair
// returns the GPOS value. It also verifies that a pair with only legacy
// kern data still comes through via the fallthrough branch.
func TestFaceKernPrefersGPOS(t *testing.T) {
	face := loadTestFace(t).(*sfntFace)

	// Force the GPOS cache to contain a known entry.
	face.gposResult = &GPOSAdjustments{
		Pairs: map[GPOSFeature]map[[2]uint16]PairAdjustment{
			GPOSKern: {
				[2]uint16{1, 2}: {XAdvance: -77},
			},
		},
	}
	face.gposParsed = true

	// Force the legacy kern cache to contain a different value for the
	// same pair plus a legacy-only pair.
	face.kernPairs = map[[2]uint16]int16{
		{1, 2}: -11,
		{3, 4}: -22,
	}
	face.kernPairsParsed = true

	if got := face.Kern(1, 2); got != -77 {
		t.Errorf("Kern(1,2) = %d, want -77 (GPOS wins over legacy kern)", got)
	}
	if got := face.Kern(3, 4); got != -22 {
		t.Errorf("Kern(3,4) = %d, want -22 (legacy kern fallback)", got)
	}
	if got := face.Kern(9, 9); got != 0 {
		t.Errorf("Kern(9,9) = %d, want 0", got)
	}
}

// cursivePosBody builds a CursivePosFormat1 subtable with two glyphs
// in coverage. glyph1 carries (entryX1, entryY1) entry / (exitX1, exitY1)
// exit; glyph2 carries entry/exit pair 2. A zero (presence == false) on
// either anchor causes that field to be emitted with offset 0 (absent).
type cursiveGlyphSpec struct {
	gid                          uint16
	hasEntry, hasExit            bool
	entryX, entryY, exitX, exitY int16
}

func cursivePosBody(specs ...cursiveGlyphSpec) []byte {
	var b gposBuilder
	b.u16(1) // posFormat
	covOffPos := b.pos()
	b.u16(0) // coverageOffset
	b.u16(uint16(len(specs)))

	// Reserve EntryExitRecords; patch their offsets later.
	type recPos struct {
		entryPos, exitPos int
	}
	recs := make([]recPos, len(specs))
	for i := range specs {
		recs[i].entryPos = b.pos()
		b.u16(0) // entryAnchorOffset
		recs[i].exitPos = b.pos()
		b.u16(0) // exitAnchorOffset
	}

	// Coverage lists glyphs in order.
	covOff := b.pos()
	b.patchU16(covOffPos, uint16(covOff))
	gids := make([]uint16, len(specs))
	for i, s := range specs {
		gids[i] = s.gid
	}
	b.buf = append(b.buf, buildCoverageFormat1(gids...)...)

	// Anchors. All offsets are from subtable start (off = 0 in the body).
	for i, s := range specs {
		if s.hasEntry {
			anchorOff := b.pos()
			b.patchU16(recs[i].entryPos, uint16(anchorOff))
			b.u16(1) // anchor format 1
			b.i16(s.entryX)
			b.i16(s.entryY)
		}
		if s.hasExit {
			anchorOff := b.pos()
			b.patchU16(recs[i].exitPos, uint16(anchorOff))
			b.u16(1)
			b.i16(s.exitX)
			b.i16(s.exitY)
		}
	}
	return b.buf
}

// TestParseGPOSCursivePosFormat1 asserts that two glyphs with entry/exit
// anchors land in the Cursives map with the right anchors and presence
// flags, and that CursiveOffset returns prev.Exit - curr.Entry.
func TestParseGPOSCursivePosFormat1(t *testing.T) {
	// Glyph 100: exit (700, 50). Glyph 101: entry (50, 60), exit (650, 40).
	body := cursivePosBody(
		cursiveGlyphSpec{gid: 100, hasExit: true, exitX: 700, exitY: 50},
		cursiveGlyphSpec{gid: 101, hasEntry: true, hasExit: true,
			entryX: 50, entryY: 60, exitX: 650, exitY: 40},
	)
	gpos := buildGPOSHeader("curs", 3, body, 0)
	g := ParseGPOS(wrapTTF(gpos))
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	cur := g.Cursives[GPOSCurs]
	if len(cur) != 2 {
		t.Fatalf("Cursives[curs] size = %d, want 2", len(cur))
	}
	rec100, ok := cur[100]
	if !ok {
		t.Fatal("missing record for glyph 100")
	}
	if rec100.HasEntry || !rec100.HasExit {
		t.Errorf("glyph 100 presence: HasEntry=%v HasExit=%v, want false true", rec100.HasEntry, rec100.HasExit)
	}
	if rec100.Exit.X != 700 || rec100.Exit.Y != 50 {
		t.Errorf("glyph 100 exit = %+v, want (700, 50)", rec100.Exit)
	}
	rec101 := cur[101]
	if !rec101.HasEntry || !rec101.HasExit {
		t.Errorf("glyph 101 presence: HasEntry=%v HasExit=%v, want both true", rec101.HasEntry, rec101.HasExit)
	}
	if rec101.Entry.X != 50 || rec101.Entry.Y != 60 {
		t.Errorf("glyph 101 entry = %+v, want (50, 60)", rec101.Entry)
	}

	// CursiveOffset(100, 101): exit (700, 50) - entry (50, 60) = (650, -10).
	dx, dy, ok := g.CursiveOffset(100, 101, GPOSCurs)
	if !ok {
		t.Fatal("CursiveOffset(100, 101) ok=false, want true")
	}
	if dx != 650 || dy != -10 {
		t.Errorf("CursiveOffset(100, 101) = (%d, %d), want (650, -10)", dx, dy)
	}

	// Glyph 100 has no entry, so (101, 100) should miss.
	if _, _, ok := g.CursiveOffset(101, 100, GPOSCurs); ok {
		t.Error("CursiveOffset(101, 100) should miss: glyph 100 has no entry anchor")
	}
	// Unknown glyphs miss.
	if _, _, ok := g.CursiveOffset(100, 999, GPOSCurs); ok {
		t.Error("CursiveOffset with unknown current glyph should miss")
	}
	if _, _, ok := g.CursiveOffset(999, 101, GPOSCurs); ok {
		t.Error("CursiveOffset with unknown previous glyph should miss")
	}
}

// TestCursiveOffsetNilReceiver covers nil-safety for the cursive path.
func TestCursiveOffsetNilReceiver(t *testing.T) {
	var g *GPOSAdjustments
	if _, _, ok := g.CursiveOffset(1, 2, GPOSCurs); ok {
		t.Error("CursiveOffset on nil receiver must return ok=false")
	}
}

// TestParseGPOSCursiveExtensionWrap verifies the type-3 dispatch follows
// LookupType 9 extension subtables, like types 2/4/6.
func TestParseGPOSCursiveExtensionWrap(t *testing.T) {
	body := cursivePosBody(
		cursiveGlyphSpec{gid: 10, hasExit: true, exitX: 100, exitY: 0},
		cursiveGlyphSpec{gid: 11, hasEntry: true, entryX: 20, entryY: 0},
	)
	gpos := buildGPOSHeader("curs", 3, body, 3)
	g := ParseGPOS(wrapTTF(gpos))
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	dx, dy, ok := g.CursiveOffset(10, 11, GPOSCurs)
	if !ok {
		t.Fatal("CursiveOffset through extension ok=false")
	}
	if dx != 80 || dy != 0 {
		t.Errorf("CursiveOffset through extension = (%d, %d), want (80, 0)", dx, dy)
	}
}

// TestCursiveOffsetCumulativeChain exercises the LTR convention across
// a three-glyph chain. With each link reusing the previous exit, the
// caller's running offset should advance by exit_n − entry_(n+1).
func TestCursiveOffsetCumulativeChain(t *testing.T) {
	body := cursivePosBody(
		// G1: exit (500, 10).
		cursiveGlyphSpec{gid: 1, hasExit: true, exitX: 500, exitY: 10},
		// G2: entry (40, -5), exit (490, 20).
		cursiveGlyphSpec{gid: 2, hasEntry: true, hasExit: true,
			entryX: 40, entryY: -5, exitX: 490, exitY: 20},
		// G3: entry (60, 15).
		cursiveGlyphSpec{gid: 3, hasEntry: true, entryX: 60, entryY: 15},
	)
	gpos := buildGPOSHeader("curs", 3, body, 0)
	g := ParseGPOS(wrapTTF(gpos))
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	// Link 1: prev=G1, curr=G2 → (500-40, 10-(-5)) = (460, 15).
	dx12, dy12, ok := g.CursiveOffset(1, 2, GPOSCurs)
	if !ok || dx12 != 460 || dy12 != 15 {
		t.Errorf("link 1 = (%d,%d,%v), want (460,15,true)", dx12, dy12, ok)
	}
	// Link 2: prev=G2, curr=G3 → (490-60, 20-15) = (430, 5).
	dx23, dy23, ok := g.CursiveOffset(2, 3, GPOSCurs)
	if !ok || dx23 != 430 || dy23 != 5 {
		t.Errorf("link 2 = (%d,%d,%v), want (430,5,true)", dx23, dy23, ok)
	}
	// Cumulative offset that the caller would apply across the run:
	// glyph 2 sits at link1; glyph 3 sits at link1 + link2.
	cumX, cumY := dx12+dx23, dy12+dy23
	if cumX != 890 || cumY != 20 {
		t.Errorf("cumulative = (%d,%d), want (890,20)", cumX, cumY)
	}
}

// markLigPosBody builds a MarkLigPosFormat1 subtable with one mark
// glyph (per markGID/class/anchor pair in marks) and one ligature glyph
// whose LigatureAttach has componentCount components × markClassCount
// mark classes; anchors[componentIdx][classIdx] supplies the (x, y) for
// each grid cell. A nil cell anchor (presence=false) emits offset 0.
type markLigMarkSpec struct {
	gid    uint16
	class  uint16
	anchor Anchor
}

type ligAnchorSpec struct {
	present bool
	x, y    int16
}

func markLigPosBody(marks []markLigMarkSpec, ligGID uint16, markClassCount int, anchors [][]ligAnchorSpec) []byte {
	var b gposBuilder
	b.u16(1) // posFormat
	markCovOffPos := b.pos()
	b.u16(0)
	ligCovOffPos := b.pos()
	b.u16(0)
	b.u16(uint16(markClassCount))
	markArrayOffPos := b.pos()
	b.u16(0)
	ligArrayOffPos := b.pos()
	b.u16(0)

	// Mark coverage in the order of the marks slice.
	markGIDs := make([]uint16, len(marks))
	for i, m := range marks {
		markGIDs[i] = m.gid
	}
	markCovOff := b.pos()
	b.patchU16(markCovOffPos, uint16(markCovOff))
	b.buf = append(b.buf, buildCoverageFormat1(markGIDs...)...)

	// Ligature coverage: a single ligature glyph.
	ligCovOff := b.pos()
	b.patchU16(ligCovOffPos, uint16(ligCovOff))
	b.buf = append(b.buf, buildCoverageFormat1(ligGID)...)

	// MarkArray.
	markArrayOff := b.pos()
	b.patchU16(markArrayOffPos, uint16(markArrayOff))
	b.u16(uint16(len(marks)))
	markAnchorOffPositions := make([]int, len(marks))
	for i, m := range marks {
		b.u16(m.class)
		markAnchorOffPositions[i] = b.pos()
		b.u16(0) // markAnchorOffset placeholder (from MarkArray start)
	}
	for i, m := range marks {
		anchorOff := b.pos()
		b.patchU16(markAnchorOffPositions[i], uint16(anchorOff-markArrayOff))
		b.u16(1)
		b.i16(m.anchor.X)
		b.i16(m.anchor.Y)
	}

	// LigatureArray: one ligature.
	ligArrayOff := b.pos()
	b.patchU16(ligArrayOffPos, uint16(ligArrayOff))
	b.u16(1) // ligatureCount
	attachOffPos := b.pos()
	b.u16(0) // ligatureAttachOffset placeholder (from LigatureArray start)

	// LigatureAttach.
	attachOff := b.pos()
	b.patchU16(attachOffPos, uint16(attachOff-ligArrayOff))
	componentCount := len(anchors)
	b.u16(uint16(componentCount))
	// Reserve the per-component anchor offset rows.
	rowAnchorPos := make([][]int, componentCount)
	for c := 0; c < componentCount; c++ {
		row := make([]int, markClassCount)
		for k := 0; k < markClassCount; k++ {
			row[k] = b.pos()
			b.u16(0) // anchor offset placeholder
		}
		rowAnchorPos[c] = row
	}
	// Emit anchors for present cells; patch their offsets relative to
	// the LigatureAttach start.
	for c := 0; c < componentCount; c++ {
		for k := 0; k < markClassCount; k++ {
			cell := anchors[c][k]
			if !cell.present {
				continue
			}
			anchorOff := b.pos()
			b.patchU16(rowAnchorPos[c][k], uint16(anchorOff-attachOff))
			b.u16(1)
			b.i16(cell.x)
			b.i16(cell.y)
		}
	}
	return b.buf
}

// TestParseGPOSMarkLigPosFormat1 covers the parser with a 2-component
// ligature carrying anchors for two mark classes. Each (component, class)
// cell is checked against MarkLigatureOffset.
func TestParseGPOSMarkLigPosFormat1(t *testing.T) {
	const (
		mark0GID uint16 = 200 // class 0 mark
		mark1GID uint16 = 201 // class 1 mark
		ligGID   uint16 = 300
	)
	marks := []markLigMarkSpec{
		{gid: mark0GID, class: 0, anchor: Anchor{X: 100, Y: 200}},
		{gid: mark1GID, class: 1, anchor: Anchor{X: 50, Y: 50}},
	}
	// 2 components × 2 mark classes.
	// Component 0: class-0 anchor (500, 800), class-1 anchor (520, 850).
	// Component 1: class-0 anchor (1500, 800), class-1 anchor absent.
	ligAnchors := [][]ligAnchorSpec{
		{
			{present: true, x: 500, y: 800},
			{present: true, x: 520, y: 850},
		},
		{
			{present: true, x: 1500, y: 800},
			{present: false},
		},
	}
	body := markLigPosBody(marks, ligGID, 2, ligAnchors)
	gpos := buildGPOSHeader("mark", 5, body, 0)
	g := ParseGPOS(wrapTTF(gpos))
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}

	lig, ok := g.LigatureBases[GPOSMark][ligGID]
	if !ok {
		t.Fatal("missing LigatureRecord for ligGID")
	}
	if len(lig.Components) != 2 {
		t.Fatalf("componentCount = %d, want 2", len(lig.Components))
	}
	if lig.Components[0][0] != (Anchor{X: 500, Y: 800}) {
		t.Errorf("comp 0 class 0 = %+v, want (500, 800)", lig.Components[0][0])
	}
	if lig.Components[0][1] != (Anchor{X: 520, Y: 850}) {
		t.Errorf("comp 0 class 1 = %+v, want (520, 850)", lig.Components[0][1])
	}
	if lig.Components[1][0] != (Anchor{X: 1500, Y: 800}) {
		t.Errorf("comp 1 class 0 = %+v, want (1500, 800)", lig.Components[1][0])
	}
	if lig.Components[1][1] != (Anchor{}) {
		t.Errorf("comp 1 class 1 should be zero anchor, got %+v", lig.Components[1][1])
	}

	cases := []struct {
		lig            uint16
		comp           int
		mark           uint16
		wantDX, wantDY int16
		wantOK         bool
	}{
		{ligGID, 0, mark0GID, 400, 600, true},  // 500-100, 800-200
		{ligGID, 0, mark1GID, 470, 800, true},  // 520-50,  850-50
		{ligGID, 1, mark0GID, 1400, 600, true}, // 1500-100, 800-200
		{ligGID, 1, mark1GID, 0, 0, false},     // class 1 absent on comp 1
		{ligGID, 2, mark0GID, 0, 0, false},     // out of range component
		{ligGID, -1, mark0GID, 0, 0, false},    // negative component
		{999, 0, mark0GID, 0, 0, false},        // unknown ligature
		{ligGID, 0, 999, 0, 0, false},          // unknown mark
	}
	for _, c := range cases {
		dx, dy, ok := g.MarkLigatureOffset(c.lig, c.comp, c.mark, GPOSMark)
		if ok != c.wantOK || (ok && (dx != c.wantDX || dy != c.wantDY)) {
			t.Errorf("MarkLigatureOffset(%d, %d, %d) = (%d, %d, %v), want (%d, %d, %v)",
				c.lig, c.comp, c.mark, dx, dy, ok, c.wantDX, c.wantDY, c.wantOK)
		}
	}
}

// TestMarkLigatureOffsetNilReceiver covers the nil-safety contract for
// the new ligature accessor.
func TestMarkLigatureOffsetNilReceiver(t *testing.T) {
	var g *GPOSAdjustments
	if _, _, ok := g.MarkLigatureOffset(1, 0, 2, GPOSMark); ok {
		t.Error("MarkLigatureOffset on nil receiver must return ok=false")
	}
}

// TestParseGPOSMarkLigExtensionWrap verifies that LookupType 5 is also
// followed through a LookupType 9 extension subtable.
func TestParseGPOSMarkLigExtensionWrap(t *testing.T) {
	const (
		markGID uint16 = 200
		ligGID  uint16 = 300
	)
	marks := []markLigMarkSpec{
		{gid: markGID, class: 0, anchor: Anchor{X: 10, Y: 20}},
	}
	ligAnchors := [][]ligAnchorSpec{
		{{present: true, x: 110, y: 120}},
		{{present: true, x: 210, y: 320}},
	}
	body := markLigPosBody(marks, ligGID, 1, ligAnchors)
	gpos := buildGPOSHeader("mark", 5, body, 5)
	g := ParseGPOS(wrapTTF(gpos))
	if g == nil {
		t.Fatal("ParseGPOS returned nil")
	}
	dx, dy, ok := g.MarkLigatureOffset(ligGID, 0, markGID, GPOSMark)
	if !ok || dx != 100 || dy != 100 {
		t.Errorf("comp 0 through extension = (%d,%d,%v), want (100,100,true)", dx, dy, ok)
	}
	dx, dy, ok = g.MarkLigatureOffset(ligGID, 1, markGID, GPOSMark)
	if !ok || dx != 200 || dy != 300 {
		t.Errorf("comp 1 through extension = (%d,%d,%v), want (200,300,true)", dx, dy, ok)
	}
}

// TestFaceGPOSCacheIdentity exercises the GPOS one-shot cache flag.
func TestFaceGPOSCacheIdentity(t *testing.T) {
	face := loadTestFace(t).(*sfntFace)
	_ = face.GPOS()
	if !face.gposParsed {
		t.Fatal("expected gposParsed=true after first GPOS call")
	}
	first := face.gposResult
	_ = face.GPOS()
	if face.gposResult != first {
		t.Error("second GPOS call rebuilt the cached result")
	}
}
