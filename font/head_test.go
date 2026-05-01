// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"errors"
	"testing"
)

// buildHeadTable returns a valid 54-byte head table populated with the
// caller-supplied unitsPerEm, bbox, and indexToLocFormat. Other
// fields are filled with zeros, the magicNumber is set correctly, and
// the version is 1.0.
func buildHeadTable(upem uint16, xMin, yMin, xMax, yMax, indexToLocFormat int16) []byte {
	buf := make([]byte, 54)
	binary.BigEndian.PutUint16(buf[0:2], 1)            // majorVersion
	binary.BigEndian.PutUint16(buf[2:4], 0)            // minorVersion
	binary.BigEndian.PutUint32(buf[12:16], 0x5F0F3CF5) // magicNumber
	binary.BigEndian.PutUint16(buf[18:20], upem)
	binary.BigEndian.PutUint16(buf[36:38], uint16(xMin))
	binary.BigEndian.PutUint16(buf[38:40], uint16(yMin))
	binary.BigEndian.PutUint16(buf[40:42], uint16(xMax))
	binary.BigEndian.PutUint16(buf[42:44], uint16(yMax))
	binary.BigEndian.PutUint16(buf[50:52], uint16(indexToLocFormat))
	return buf
}

func TestParseHeadSynthetic(t *testing.T) {
	data := buildHeadTable(2048, -100, -200, 1000, 800, 1)
	h, err := parseHead(data)
	if err != nil {
		t.Fatalf("parseHead: %v", err)
	}
	if h.unitsPerEm != 2048 {
		t.Errorf("unitsPerEm = %d, want 2048", h.unitsPerEm)
	}
	if h.xMin != -100 || h.yMin != -200 || h.xMax != 1000 || h.yMax != 800 {
		t.Errorf("bbox = (%d,%d)-(%d,%d), want (-100,-200)-(1000,800)", h.xMin, h.yMin, h.xMax, h.yMax)
	}
	if h.indexToLocFormat != 1 {
		t.Errorf("indexToLocFormat = %d, want 1", h.indexToLocFormat)
	}
}

func TestParseHeadTruncated(t *testing.T) {
	data := buildHeadTable(1000, 0, 0, 1, 1, 0)
	_, err := parseHead(data[:30])
	if err == nil {
		t.Fatal("expected error for truncated head")
	}
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want errors.Is ErrTruncated", err)
	}
}

func TestParseHeadBadMagic(t *testing.T) {
	data := buildHeadTable(1000, 0, 0, 1, 1, 0)
	binary.BigEndian.PutUint32(data[12:16], 0xDEADBEEF)
	_, err := parseHead(data)
	if err == nil {
		t.Fatal("expected error for wrong magic")
	}
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want errors.Is ErrCorruptTable", err)
	}
}

func TestParseHeadZeroUnitsPerEm(t *testing.T) {
	data := buildHeadTable(0, 0, 0, 1, 1, 0)
	_, err := parseHead(data)
	if err == nil {
		t.Fatal("expected error for unitsPerEm = 0")
	}
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want errors.Is ErrCorruptTable", err)
	}
}

// TestParseHeadOnRealSystemFont opportunistically validates the parser
// against a real on-host font so that the parser's spec readings are
// pinned against at least one shipped TrueType.
func TestParseHeadOnRealSystemFont(t *testing.T) {
	data := loadAnySystemTTF(t)
	tables, err := parseTTFTables(data)
	if err != nil {
		t.Fatalf("parseTTFTables: %v", err)
	}
	headData, ok := tables["head"]
	if !ok {
		t.Skip("system font has no head table")
	}
	h, err := parseHead(headData)
	if err != nil {
		t.Fatalf("parseHead on system font: %v", err)
	}
	if h.unitsPerEm == 0 {
		t.Error("system font reports unitsPerEm = 0")
	}
	if h.xMin >= h.xMax || h.yMin >= h.yMax {
		t.Errorf("system font bbox is degenerate: %v", h)
	}
}
