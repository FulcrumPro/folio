// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"errors"
	"testing"
)

// buildMaxpV05 builds the 6-byte version-0.5 maxp table (CFF outline
// fonts). buildMaxpV1 builds the 32-byte v1.0 layout (TrueType
// outlines); only numGlyphs is consulted by the parser.
func buildMaxpV05(numGlyphs uint16) []byte {
	buf := make([]byte, 6)
	binary.BigEndian.PutUint32(buf[0:4], 0x00005000)
	binary.BigEndian.PutUint16(buf[4:6], numGlyphs)
	return buf
}

func buildMaxpV1(numGlyphs uint16) []byte {
	buf := make([]byte, 32)
	binary.BigEndian.PutUint32(buf[0:4], 0x00010000)
	binary.BigEndian.PutUint16(buf[4:6], numGlyphs)
	return buf
}

func TestParseMaxpV05(t *testing.T) {
	m, err := parseMaxp(buildMaxpV05(257))
	if err != nil {
		t.Fatalf("parseMaxp v0.5: %v", err)
	}
	if m.numGlyphs != 257 {
		t.Errorf("numGlyphs = %d, want 257", m.numGlyphs)
	}
}

func TestParseMaxpV1(t *testing.T) {
	m, err := parseMaxp(buildMaxpV1(40000))
	if err != nil {
		t.Fatalf("parseMaxp v1: %v", err)
	}
	if m.numGlyphs != 40000 {
		t.Errorf("numGlyphs = %d, want 40000", m.numGlyphs)
	}
}

func TestParseMaxpTruncated(t *testing.T) {
	_, err := parseMaxp([]byte{0, 0, 1, 0, 1}) // 5 bytes < 6
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want errors.Is ErrTruncated", err)
	}
}

func TestParseMaxpZeroGlyphs(t *testing.T) {
	_, err := parseMaxp(buildMaxpV05(0))
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want errors.Is ErrCorruptTable", err)
	}
}

func TestParseMaxpOnRealSystemFont(t *testing.T) {
	data := loadAnySystemTTF(t)
	tables, err := parseTTFTables(data)
	if err != nil {
		t.Fatalf("parseTTFTables: %v", err)
	}
	maxpData, ok := tables["maxp"]
	if !ok {
		t.Skip("system font has no maxp table")
	}
	m, err := parseMaxp(maxpData)
	if err != nil {
		t.Fatalf("parseMaxp on system font: %v", err)
	}
	if m.numGlyphs == 0 {
		t.Error("real font has numGlyphs = 0")
	}
}
