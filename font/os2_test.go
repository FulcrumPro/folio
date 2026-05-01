// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"errors"
	"testing"
)

// buildOS2 builds an OS/2 table at the requested version. Versions
// 0 and 1 produce 78 bytes; v2+ produces 96 bytes (the v4 prefix). The
// caller-supplied metric fields populate the well-known offsets;
// other fields are zero-filled.
func buildOS2(version uint16, fsSelection uint16, typoAsc, typoDesc, typoLineGap int16, winAsc, winDesc uint16, sxHeight, sCapHeight int16) []byte {
	size := 78
	if version >= 2 {
		size = 96
	}
	buf := make([]byte, size)
	binary.BigEndian.PutUint16(buf[0:2], version)
	binary.BigEndian.PutUint16(buf[62:64], fsSelection)
	binary.BigEndian.PutUint16(buf[68:70], uint16(typoAsc))
	binary.BigEndian.PutUint16(buf[70:72], uint16(typoDesc))
	binary.BigEndian.PutUint16(buf[72:74], uint16(typoLineGap))
	binary.BigEndian.PutUint16(buf[74:76], winAsc)
	binary.BigEndian.PutUint16(buf[76:78], winDesc)
	if version >= 2 {
		binary.BigEndian.PutUint16(buf[86:88], uint16(sxHeight))
		binary.BigEndian.PutUint16(buf[88:90], uint16(sCapHeight))
	}
	return buf
}

func TestParseOS2V0(t *testing.T) {
	data := buildOS2(0, 0x0040 /*REGULAR*/, 1900, -500, 0, 1900, 500, 0, 0)
	o, err := parseOS2(data)
	if err != nil {
		t.Fatalf("parseOS2 v0: %v", err)
	}
	if o.version != 0 {
		t.Errorf("version = %d, want 0", o.version)
	}
	if o.sTypoAscender != 1900 || o.sTypoDescender != -500 {
		t.Errorf("typo metrics = (%d,%d), want (1900,-500)", o.sTypoAscender, o.sTypoDescender)
	}
	if o.sCapHeight != 0 || o.sxHeight != 0 {
		t.Errorf("v0 should leave sCapHeight/sxHeight zero, got (%d,%d)", o.sCapHeight, o.sxHeight)
	}
}

func TestParseOS2V2(t *testing.T) {
	data := buildOS2(2, 0x00C0 /*REGULAR + USE_TYPO_METRICS*/, 1900, -500, 100, 2000, 500, 1100, 1480)
	o, err := parseOS2(data)
	if err != nil {
		t.Fatalf("parseOS2 v2: %v", err)
	}
	if !o.useTypoMetrics() {
		t.Error("USE_TYPO_METRICS bit not detected")
	}
	if o.sCapHeight != 1480 {
		t.Errorf("sCapHeight = %d, want 1480", o.sCapHeight)
	}
	if o.sxHeight != 1100 {
		t.Errorf("sxHeight = %d, want 1100", o.sxHeight)
	}
}

func TestParseOS2Truncated(t *testing.T) {
	_, err := parseOS2(make([]byte, 50))
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want errors.Is ErrTruncated", err)
	}
}

// TestSfntFaceUsesTypoMetricsWhenBitSet pins the Face-level wiring of
// the USE_TYPO_METRICS bit. The unit-level os2_test.go cases verify
// the parser reads the bit correctly; this test verifies that
// sfntFace.Ascent() / Descent() actually consult it.
//
// No system font on macOS, Linux, or Windows ships with the bit set
// in its standard system fonts (Calibri/Aptos and post-2017 Microsoft
// foundry fonts do, but they aren't on dev/CI hosts as system fonts).
// Without this test, the USE_TYPO_METRICS-set branch in Ascent /
// Descent has zero CI coverage on any host — a regression that
// inverted the condition would only surface for end users with
// post-2017 Microsoft fonts.
//
// The test takes any system TTF, surgically patches its OS/2 table to
// (a) set USE_TYPO_METRICS (fsSelection bit 7) and (b) shift
// sTypoAscender by a distinguishable delta, then asserts the patched
// font's Ascent reflects the typo value. As a control, it also
// patches only sTypoAscender (without setting the bit) and asserts
// Ascent stays at the hhea baseline — proving the bit gates the
// choice.
func TestSfntFaceUsesTypoMetricsWhenBitSet(t *testing.T) {
	ttf := loadAnySystemTTF(t)

	// Baseline: parse the font as-is. Ascent should be hhea.ascender
	// since real system fonts almost universally have USE_TYPO_METRICS
	// unset.
	baseFace, err := ParseTTF(ttf)
	if err != nil {
		t.Fatalf("baseline ParseTTF: %v", err)
	}
	baseAscent := baseFace.Ascent()
	const delta = 999

	// Patch OS/2: set USE_TYPO_METRICS bit AND bump sTypoAscender so we
	// can distinguish "typo path was taken" (Ascent == base + delta)
	// from "hhea path was taken" (Ascent == base).
	patched := patchOS2(t, ttf, true, int16(baseAscent+delta))
	patchedFace, err := ParseTTF(patched)
	if err != nil {
		t.Fatalf("patched ParseTTF: %v", err)
	}
	if got := patchedFace.Ascent(); got != baseAscent+delta {
		t.Errorf("USE_TYPO_METRICS set: Ascent = %d, want %d (hhea baseline %d + delta %d). Bit-gated typo selection regressed.",
			got, baseAscent+delta, baseAscent, delta)
	}

	// Control: bump sTypoAscender WITHOUT setting the bit. Ascent must
	// stay at hhea baseline. Proves the bit is what gates the choice,
	// not just OS/2 presence.
	control := patchOS2(t, ttf, false, int16(baseAscent+delta))
	controlFace, err := ParseTTF(control)
	if err != nil {
		t.Fatalf("control ParseTTF: %v", err)
	}
	if got := controlFace.Ascent(); got != baseAscent {
		t.Errorf("USE_TYPO_METRICS unset: Ascent = %d, want %d (hhea baseline). Selection ignored the bit.",
			got, baseAscent)
	}
}

// patchOS2 mutates a copy of ttf's OS/2 table in place: when
// setTypoBit is true, fsSelection's bit 7 is set; sTypoAscender is
// always overwritten with newTypoAsc. Other OS/2 fields are
// preserved. fsSelection is at OS/2 offset 62 (uint16);
// sTypoAscender is at offset 68 (int16).
func patchOS2(t *testing.T, ttf []byte, setTypoBit bool, newTypoAsc int16) []byte {
	t.Helper()
	out := make([]byte, len(ttf))
	copy(out, ttf)
	numTables := int(binary.BigEndian.Uint16(out[4:6]))
	for i := 0; i < numTables; i++ {
		entry := 12 + i*16
		if string(out[entry:entry+4]) != "OS/2" {
			continue
		}
		off := int(binary.BigEndian.Uint32(out[entry+8 : entry+12]))
		length := int(binary.BigEndian.Uint32(out[entry+12 : entry+16]))
		if off+78 > len(out) || length < 78 {
			t.Fatalf("OS/2 table too short: off=%d len=%d", off, length)
		}
		if setTypoBit {
			fsSel := binary.BigEndian.Uint16(out[off+62 : off+64])
			binary.BigEndian.PutUint16(out[off+62:off+64], fsSel|0x80)
		}
		binary.BigEndian.PutUint16(out[off+68:off+70], uint16(newTypoAsc))
		return out
	}
	t.Fatal("ttf has no OS/2 table to patch")
	return nil
}

func TestParseOS2OnRealSystemFont(t *testing.T) {
	data := loadAnySystemTTF(t)
	tables, err := parseTTFTables(data)
	if err != nil {
		t.Fatalf("parseTTFTables: %v", err)
	}
	os2Data, ok := tables["OS/2"]
	if !ok {
		t.Skip("system font has no OS/2 table")
	}
	o, err := parseOS2(os2Data)
	if err != nil {
		t.Fatalf("parseOS2 on system font: %v", err)
	}
	if o.sTypoAscender <= 0 {
		t.Errorf("real font sTypoAscender = %d, expected positive", o.sTypoAscender)
	}
}
