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
