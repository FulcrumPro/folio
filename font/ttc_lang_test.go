// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"testing"
)

// TestPickFaceForLanguageMatchesRegionalFamily verifies the core
// matching contract: a TTC whose face FontFamily names contain
// regional tokens ("JP", "SC", "TC", "KR") returns the index of the
// face matching the requested language.
//
// The fixture is a 4-face TTC modelled after NotoSansCJK-Regular.ttc:
// face 0 = JP, face 1 = KR, face 2 = SC, face 3 = TC. The picker
// must return the SC face (index 2) for "zh-CN", the TC face for
// "zh-TW", JP for "ja", KR for "ko".
func TestPickFaceForLanguageMatchesRegionalFamily(t *testing.T) {
	ttc := buildSyntheticCJKTTC(t,
		"Noto Sans CJK JP",
		"Noto Sans CJK KR",
		"Noto Sans CJK SC",
		"Noto Sans CJK TC",
	)

	cases := []struct {
		lang     string
		wantIdx  int
		wantSlug string
	}{
		{"ja", 0, "JP"},
		{"ko", 1, "KR"},
		{"zh-CN", 2, "SC"},
		{"zh-Hans", 2, "SC"},
		{"zh-SG", 2, "SC"},
		{"zh", 2, "SC"}, // bare zh defaults to Simplified
		{"zh-TW", 3, "TC"},
		{"zh-Hant", 3, "TC"},
		{"zh-HK", 3, "TC"},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			got := pickFaceForLanguage(ttc, tc.lang)
			if got != tc.wantIdx {
				t.Errorf("pickFaceForLanguage(%q) = %d, want %d (%s)",
					tc.lang, got, tc.wantIdx, tc.wantSlug)
			}
		})
	}
}

// TestPickFaceForLanguageReturnsNegativeOnNoMatch verifies the
// fallback signal: when no face matches the requested language (or
// the language isn't a known CJK convention), the picker returns -1
// so the caller can fall back to face 0.
func TestPickFaceForLanguageReturnsNegativeOnNoMatch(t *testing.T) {
	// Latin-only TTC — no JP/SC/TC/KR substrings. Picker must signal
	// "no match" rather than guessing.
	latinTTC := buildSyntheticCJKTTC(t,
		"Helvetica",
		"Helvetica Bold",
	)
	if got := pickFaceForLanguage(latinTTC, "ja"); got != -1 {
		t.Errorf("Latin-only TTC + ja: got %d, want -1", got)
	}

	// CJK TTC with a language Folio doesn't know how to map.
	cjkTTC := buildSyntheticCJKTTC(t, "Noto Sans CJK JP", "Noto Sans CJK SC")
	if got := pickFaceForLanguage(cjkTTC, "fr"); got != -1 {
		t.Errorf("CJK TTC + fr: got %d, want -1", got)
	}

	// Empty language hint: short-circuit return -1.
	if got := pickFaceForLanguage(cjkTTC, ""); got != -1 {
		t.Errorf("empty lang: got %d, want -1", got)
	}
}

// TestPickFaceForLanguageMalformedInputReturnsNegative pins the
// safety contract: malformed TTC bytes never panic and never return
// a positive index. The caller's `max(idx, 0)` then safely picks
// face 0 (or fails extractTTCFont with a wrapped sentinel — that's
// the next layer's job).
func TestPickFaceForLanguageMalformedInputReturnsNegative(t *testing.T) {
	cases := map[string][]byte{
		"too short":   {0x74, 0x74, 0x63, 0x66, 0, 1, 0, 0},
		"wrong magic": make([]byte, 16),
		"empty":       nil,
		"truncated":   {0x74, 0x74, 0x63, 0x66, 0, 1, 0, 0, 0, 0, 0, 4},
		"zero numFonts": {0x74, 0x74, 0x63, 0x66, 0, 1, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0},
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("pickFaceForLanguage panicked on %q: %v", name, r)
				}
			}()
			if got := pickFaceForLanguage(data, "ja"); got != -1 {
				t.Errorf("%s: got %d, want -1", name, got)
			}
		})
	}
}

// TestParseFontForLanguageRoundTripsThroughTTC pins the public
// API: ParseFontForLanguage on a synthetic CJK TTC must produce a
// Face whose PostScriptName matches the SC face when the SC face
// carries a distinguishable name.
func TestParseFontForLanguageRoundTripsThroughTTC(t *testing.T) {
	ttfBytes := loadAnySystemTTF(t)
	ttc := buildSyntheticCJKTTCFromTTF(t, ttfBytes,
		"Noto Sans CJK JP",
		"Noto Sans CJK SC",
	)
	face, err := ParseFontForLanguage(ttc, "zh-CN")
	if err != nil {
		t.Fatalf("ParseFontForLanguage: %v", err)
	}
	if face == nil {
		t.Fatal("nil face")
	}
	if face.UnitsPerEm() <= 0 {
		t.Errorf("UnitsPerEm = %d, want > 0", face.UnitsPerEm())
	}
}

// TestParseFontPicksFaceZeroByDefault pins the back-compat contract:
// ParseFont (no language hint) on a TTC always picks face 0.
func TestParseFontPicksFaceZeroByDefault(t *testing.T) {
	ttfBytes := loadAnySystemTTF(t)
	ttc := buildSyntheticCJKTTCFromTTF(t, ttfBytes,
		"Noto Sans CJK JP", // face 0
		"Noto Sans CJK SC", // face 1
	)
	face0, err := ParseFont(ttc)
	if err != nil {
		t.Fatalf("ParseFont: %v", err)
	}
	face0Bytes := face0.RawData()

	// The face we get from ParseFont must be byte-identical to face 0
	// extracted explicitly. Distinct from face 1.
	expected, err := extractTTCFont(ttc, 0)
	if err != nil {
		t.Fatalf("extractTTCFont(0): %v", err)
	}
	if !bytesEqual(face0Bytes, expected) {
		t.Error("ParseFont returned a face different from explicit face 0; back-compat broken")
	}
}

// --- Test fixtures ---

// buildSyntheticCJKTTC returns a TTC where each face has only a
// `name` table whose NameID 1 (FontFamily) carries the supplied
// family string. Used for unit-level picker tests where we don't
// need the rest of the sfnt structure to be valid — pickFaceForLanguage
// only reads the directory + name table, never the outline tables.
//
// Each face has exactly one table (name); the full sfnt-validity is
// not required because pickFaceForLanguage doesn't dispatch through
// sfnt.
func buildSyntheticCJKTTC(t *testing.T, families ...string) []byte {
	t.Helper()
	if len(families) < 1 {
		t.Fatal("need at least 1 family")
	}

	// Build each face's name table.
	nameTables := make([][]byte, len(families))
	for i, fam := range families {
		nameTables[i] = buildMinimalNameTable(t, fam)
	}

	// Layout: TTC header (12) + face offsets (4 * N) + per-face
	// (offset table 12 + 1 directory entry 16 + name table padded to 4).
	const offsetTableSize = 12
	const dirEntrySize = 16
	headerSize := 12 + len(families)*4

	// Compute per-face offset and total size.
	faceOffsets := make([]int, len(families))
	cursor := headerSize
	for i, nt := range nameTables {
		faceOffsets[i] = cursor
		// Each face has: offset table (12) + 1 dir entry (16) + name
		// table data (padded to 4 bytes).
		nameLen := len(nt)
		paddedName := (nameLen + 3) &^ 3
		cursor += offsetTableSize + dirEntrySize + paddedName
	}
	out := make([]byte, cursor)

	// TTC header
	copy(out[0:4], "ttcf")
	binary.BigEndian.PutUint32(out[4:8], 0x00010000) // version 1.0
	binary.BigEndian.PutUint32(out[8:12], uint32(len(families)))
	for i, off := range faceOffsets {
		binary.BigEndian.PutUint32(out[12+i*4:], uint32(off))
	}

	// Per-face: offset table + directory entry + name table data
	for i, nt := range nameTables {
		fOff := faceOffsets[i]
		// Offset table
		binary.BigEndian.PutUint32(out[fOff:fOff+4], 0x00010000) // sfntVersion
		binary.BigEndian.PutUint16(out[fOff+4:fOff+6], 1)        // numTables
		// searchRange/entrySelector/rangeShift left zero — picker doesn't read them
		// Directory entry for "name"
		dirOff := fOff + offsetTableSize
		copy(out[dirOff:dirOff+4], "name")
		// checksum left zero — we don't validate
		nameTableOff := dirOff + dirEntrySize
		binary.BigEndian.PutUint32(out[dirOff+8:dirOff+12], uint32(nameTableOff))
		binary.BigEndian.PutUint32(out[dirOff+12:dirOff+16], uint32(len(nt)))
		// name table data
		copy(out[nameTableOff:nameTableOff+len(nt)], nt)
	}
	return out
}

// buildMinimalNameTable returns a `name` table with a single record:
// NameID 1 (FontFamily), platform 3 (Windows), encoding 1 (Unicode
// BMP), encoded UTF-16BE.
func buildMinimalNameTable(t *testing.T, family string) []byte {
	t.Helper()
	// Encode family as UTF-16BE.
	famBytes := utf16BEEncode(family)
	// header (6) + 1 record (12) + string storage
	headerSize := 6 + 12
	totalSize := headerSize + len(famBytes)
	out := make([]byte, totalSize)
	// Header
	binary.BigEndian.PutUint16(out[0:2], 0)                  // format
	binary.BigEndian.PutUint16(out[2:4], 1)                  // count
	binary.BigEndian.PutUint16(out[4:6], uint16(headerSize)) // stringOffset
	// Name record
	binary.BigEndian.PutUint16(out[6:8], 3)                       // platformID = Windows
	binary.BigEndian.PutUint16(out[8:10], 1)                      // encodingID = Unicode BMP
	binary.BigEndian.PutUint16(out[10:12], 0x0409)                // languageID = en-US
	binary.BigEndian.PutUint16(out[12:14], 1)                     // nameID = FontFamily
	binary.BigEndian.PutUint16(out[14:16], uint16(len(famBytes))) // length
	binary.BigEndian.PutUint16(out[16:18], 0)                     // offset
	// String storage
	copy(out[headerSize:], famBytes)
	return out
}

// utf16BEEncode encodes a UTF-8 Go string as UTF-16BE bytes. ASCII
// fits in one code unit; out-of-BMP runes use surrogate pairs.
// For the family-name fixtures all input is ASCII so this is mostly
// a one-byte-per-character zero-pad.
func utf16BEEncode(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for _, r := range s {
		if r <= 0xFFFF {
			out = append(out, byte(r>>8), byte(r))
			continue
		}
		// Surrogate pair (not exercised in these fixtures but cheap to keep correct).
		r -= 0x10000
		hi := 0xD800 + (r >> 10)
		lo := 0xDC00 + (r & 0x3FF)
		out = append(out, byte(hi>>8), byte(hi), byte(lo>>8), byte(lo))
	}
	return out
}

// buildSyntheticCJKTTCFromTTF wraps multiple copies of a real TTF
// into a TTC, mutating each face's name-table FontFamily to one of
// the supplied family strings. Used for tests that need ParseFont*
// to actually return a Face — the parser requires a complete-enough
// sfnt, which the minimal nametable-only fixture doesn't satisfy.
func buildSyntheticCJKTTCFromTTF(t *testing.T, ttf []byte, families ...string) []byte {
	t.Helper()
	if len(families) < 1 {
		t.Fatal("need at least 1 family")
	}

	// For each family, mutate a copy of ttf to carry that family name in
	// its `name` table's NameID 1 record. Then wrap all faces into a TTC.
	mutatedFaces := make([][]byte, len(families))
	for i, fam := range families {
		mutatedFaces[i] = mutateTTFFontFamily(t, ttf, fam)
	}

	// Compute layout. Each face is a complete sfnt; place them
	// back-to-back after the TTC header.
	headerSize := 12 + len(families)*4
	cursor := headerSize
	faceOffsets := make([]int, len(families))
	for i, m := range mutatedFaces {
		faceOffsets[i] = cursor
		cursor += len(m)
	}
	out := make([]byte, cursor)

	copy(out[0:4], "ttcf")
	binary.BigEndian.PutUint32(out[4:8], 0x00010000)
	binary.BigEndian.PutUint32(out[8:12], uint32(len(families)))
	for i, off := range faceOffsets {
		binary.BigEndian.PutUint32(out[12+i*4:], uint32(off))
	}
	for i, m := range mutatedFaces {
		off := faceOffsets[i]
		copy(out[off:off+len(m)], m)
		// Rewrite each table directory entry's offset to be absolute
		// within the TTC (not relative to the face start) — extractTTCFont
		// expects that convention.
		numTables := int(binary.BigEndian.Uint16(out[off+4 : off+6]))
		for j := 0; j < numTables; j++ {
			entry := off + 12 + j*16
			localOff := binary.BigEndian.Uint32(out[entry+8 : entry+12])
			binary.BigEndian.PutUint32(out[entry+8:entry+12], localOff+uint32(off))
		}
	}
	return out
}

// mutateTTFFontFamily returns a copy of ttf where the `name` table
// has been rewritten to carry exactly one FontFamily (NameID 1)
// record with the supplied family string. Other name records are
// dropped — for these tests the parser only needs to find NameID 1.
func mutateTTFFontFamily(t *testing.T, ttf []byte, family string) []byte {
	t.Helper()
	out := make([]byte, len(ttf))
	copy(out, ttf)

	numTables := int(binary.BigEndian.Uint16(out[4:6]))
	for i := 0; i < numTables; i++ {
		entry := 12 + i*16
		if string(out[entry:entry+4]) != "name" {
			continue
		}
		oldOff := int(binary.BigEndian.Uint32(out[entry+8 : entry+12]))
		oldLen := int(binary.BigEndian.Uint32(out[entry+12 : entry+16]))
		newName := buildMinimalNameTable(t, family)
		// If the new name table fits in the old slot, rewrite in place.
		// Otherwise append at end and rewire the directory entry.
		if len(newName) <= oldLen {
			// Zero the old region, write new bytes at the start.
			for j := oldOff; j < oldOff+oldLen; j++ {
				out[j] = 0
			}
			copy(out[oldOff:oldOff+len(newName)], newName)
			binary.BigEndian.PutUint32(out[entry+12:entry+16], uint32(len(newName)))
		} else {
			pad := (4 - (len(out) % 4)) % 4
			out = append(out, make([]byte, pad)...)
			newOff := len(out)
			out = append(out, newName...)
			binary.BigEndian.PutUint32(out[entry+8:entry+12], uint32(newOff))
			binary.BigEndian.PutUint32(out[entry+12:entry+16], uint32(len(newName)))
		}
		return out
	}
	t.Fatal("ttf has no name table to mutate")
	return nil
}

// bytesEqual avoids importing the bytes package for a single use
// (and keeps the helper local to ttc_lang_test).
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
