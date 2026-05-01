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
