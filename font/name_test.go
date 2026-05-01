// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"errors"
	"testing"
)

// nameRec describes one record's worth of inputs to buildNameTable.
type nameRec struct {
	platformID, encodingID, languageID, nameID uint16
	value                                      []byte // raw bytes already in the record's chosen encoding
}

// buildNameTable serialises a Format-0 name table from the given
// records. String storage starts immediately after the record array.
func buildNameTable(recs []nameRec) []byte {
	header := 6
	recordsLen := len(recs) * 12
	stringStart := header + recordsLen

	// Compute total size and per-record offsets within the storage.
	var stringSize int
	offsets := make([]int, len(recs))
	for i, r := range recs {
		offsets[i] = stringSize
		stringSize += len(r.value)
	}
	buf := make([]byte, stringStart+stringSize)
	binary.BigEndian.PutUint16(buf[0:2], 0) // format
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(recs)))
	binary.BigEndian.PutUint16(buf[4:6], uint16(stringStart))

	for i, r := range recs {
		off := header + i*12
		binary.BigEndian.PutUint16(buf[off:], r.platformID)
		binary.BigEndian.PutUint16(buf[off+2:], r.encodingID)
		binary.BigEndian.PutUint16(buf[off+4:], r.languageID)
		binary.BigEndian.PutUint16(buf[off+6:], r.nameID)
		binary.BigEndian.PutUint16(buf[off+8:], uint16(len(r.value)))
		binary.BigEndian.PutUint16(buf[off+10:], uint16(offsets[i]))
		copy(buf[stringStart+offsets[i]:], r.value)
	}
	return buf
}

// utf16BE converts an ASCII or BMP string to UTF-16 big-endian bytes.
// Used by tests to mint platform-3 records.
func utf16BE(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for _, r := range s {
		// Naive BMP-only encoding suffices for the test strings.
		out = append(out, byte(r>>8), byte(r&0xFF))
	}
	return out
}

func TestParseNameWindowsBMP(t *testing.T) {
	data := buildNameTable([]nameRec{
		{platformID: 3, encodingID: 1, languageID: 0x409, nameID: nameIDPostScript, value: utf16BE("MyFont-Regular")},
		{platformID: 3, encodingID: 1, languageID: 0x409, nameID: nameIDFull, value: utf16BE("My Font Regular")},
	})
	nm, err := parseName(data)
	if err != nil {
		t.Fatalf("parseName: %v", err)
	}
	if nm.postScript != "MyFont-Regular" {
		t.Errorf("postScript = %q, want %q", nm.postScript, "MyFont-Regular")
	}
	if nm.full != "My Font Regular" {
		t.Errorf("full = %q, want %q", nm.full, "My Font Regular")
	}
}

func TestParseNamePrefersWindowsOverMacRoman(t *testing.T) {
	// Mac Roman record present first; Windows record present second.
	// The Windows record (higher score) must win.
	data := buildNameTable([]nameRec{
		{platformID: 1, encodingID: 0, nameID: nameIDPostScript, value: []byte("WrongFont")},
		{platformID: 3, encodingID: 1, nameID: nameIDPostScript, value: utf16BE("RightFont")},
	})
	nm, err := parseName(data)
	if err != nil {
		t.Fatalf("parseName: %v", err)
	}
	if nm.postScript != "RightFont" {
		t.Errorf("postScript = %q, want %q (Windows record should outrank Mac)", nm.postScript, "RightFont")
	}
}

func TestParseNameMacRomanFallback(t *testing.T) {
	// Only a Mac Roman record present; the parser must decode it.
	// Use a high-byte to exercise the macRomanHigh table: 0xA9 → ©.
	data := buildNameTable([]nameRec{
		{platformID: 1, encodingID: 0, nameID: nameIDFull, value: []byte("Mac\xA9")},
	})
	nm, err := parseName(data)
	if err != nil {
		t.Fatalf("parseName: %v", err)
	}
	if nm.full != "Mac©" {
		t.Errorf("full = %q, want %q", nm.full, "Mac©")
	}
}

func TestParseNameUnicodePlatform(t *testing.T) {
	data := buildNameTable([]nameRec{
		{platformID: 0, encodingID: 3, nameID: nameIDPostScript, value: utf16BE("UnicodeName")},
	})
	nm, err := parseName(data)
	if err != nil {
		t.Fatalf("parseName: %v", err)
	}
	if nm.postScript != "UnicodeName" {
		t.Errorf("postScript = %q, want UnicodeName", nm.postScript)
	}
}

func TestParseNameSurrogatePair(t *testing.T) {
	// U+1F600 (grinning face emoji) = surrogate pair D83D DE00.
	data := buildNameTable([]nameRec{
		{platformID: 3, encodingID: 1, nameID: nameIDFull, value: []byte{0xD8, 0x3D, 0xDE, 0x00}},
	})
	nm, err := parseName(data)
	if err != nil {
		t.Fatalf("parseName: %v", err)
	}
	if nm.full != "\U0001F600" {
		t.Errorf("full = %q, want grinning face emoji", nm.full)
	}
}

func TestParseNameTruncatedHeader(t *testing.T) {
	_, err := parseName([]byte{0, 0, 0})
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want errors.Is ErrTruncated", err)
	}
}

func TestParseNameTruncatedRecords(t *testing.T) {
	// Header claims 5 records but only 1 record's worth of bytes.
	buf := make([]byte, 6+12)
	binary.BigEndian.PutUint16(buf[2:4], 5) // count
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(buf)))
	_, err := parseName(buf)
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want errors.Is ErrTruncated", err)
	}
}

func TestParseNameSkipsMalformedRecord(t *testing.T) {
	// Two records: first has out-of-range string offset (skipped),
	// second is well-formed and should still be returned.
	data := buildNameTable([]nameRec{
		{platformID: 3, encodingID: 1, nameID: nameIDPostScript, value: utf16BE("Good")},
	})
	// Manually patch a second record with a wild stringOffset, leaving
	// the well-formed first one in place.
	patched := make([]byte, len(data)+12)
	copy(patched, data)
	binary.BigEndian.PutUint16(patched[2:4], 2) // count = 2 now
	off := 6 + 12                               // location of the new record
	binary.BigEndian.PutUint16(patched[off:], 3)
	binary.BigEndian.PutUint16(patched[off+2:], 1)
	binary.BigEndian.PutUint16(patched[off+6:], nameIDFull)
	binary.BigEndian.PutUint16(patched[off+8:], 0xFFFF) // length
	binary.BigEndian.PutUint16(patched[off+10:], 0xFF00)
	// String storage starts at the end of the record array; shift
	// by 12 to account for the appended record.
	binary.BigEndian.PutUint16(patched[4:6], uint16(6+2*12))
	// Rewrite the original record's string offset (8) so it remains
	// relative to the (new) string storage start.
	binary.BigEndian.PutUint16(patched[6+10:], 0)
	copy(patched[6+2*12:], []byte{0, 'G', 0, 'o', 0, 'o', 0, 'd'})
	binary.BigEndian.PutUint16(patched[6+8:], 8) // length=8
	nm, err := parseName(patched)
	if err != nil {
		t.Fatalf("parseName: %v", err)
	}
	if nm.postScript != "Good" {
		t.Errorf("postScript = %q, want Good", nm.postScript)
	}
	// nm.full should remain empty because the malformed record was skipped.
	if nm.full != "" {
		t.Errorf("full = %q, want empty (malformed record should be skipped)", nm.full)
	}
}

func TestParseNameOnRealSystemFont(t *testing.T) {
	data := loadAnySystemTTF(t)
	tables, err := parseTTFTables(data)
	if err != nil {
		t.Fatalf("parseTTFTables: %v", err)
	}
	nameData, ok := tables["name"]
	if !ok {
		t.Skip("system font has no name table")
	}
	nm, err := parseName(nameData)
	if err != nil {
		t.Fatalf("parseName: %v", err)
	}
	if nm.postScript == "" && nm.full == "" {
		t.Error("real system font produced empty names")
	}
}
