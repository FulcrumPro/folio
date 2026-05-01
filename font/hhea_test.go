// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"errors"
	"testing"
)

// buildHheaTable returns a valid 36-byte hhea table with the
// caller-supplied ascender, descender, lineGap, and numberOfHMetrics.
// All other fields are zero. metricDataFormat is left as 0 (the only
// valid value per §5.2.3).
func buildHheaTable(asc, desc, lineGap int16, numH uint16) []byte {
	buf := make([]byte, 36)
	binary.BigEndian.PutUint16(buf[0:2], 1) // majorVersion
	binary.BigEndian.PutUint16(buf[4:6], uint16(asc))
	binary.BigEndian.PutUint16(buf[6:8], uint16(desc))
	binary.BigEndian.PutUint16(buf[8:10], uint16(lineGap))
	binary.BigEndian.PutUint16(buf[34:36], numH)
	return buf
}

func TestParseHheaSynthetic(t *testing.T) {
	data := buildHheaTable(1900, -500, 0, 256)
	h, err := parseHhea(data)
	if err != nil {
		t.Fatalf("parseHhea: %v", err)
	}
	if h.ascender != 1900 || h.descender != -500 || h.lineGap != 0 {
		t.Errorf("hhea metrics = %+v, want asc=1900 desc=-500 gap=0", h)
	}
	if h.numberOfHMetrics != 256 {
		t.Errorf("numberOfHMetrics = %d, want 256", h.numberOfHMetrics)
	}
}

func TestParseHheaTruncated(t *testing.T) {
	data := buildHheaTable(0, 0, 0, 1)
	_, err := parseHhea(data[:20])
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want errors.Is ErrTruncated", err)
	}
}

func TestParseHheaOnRealSystemFont(t *testing.T) {
	data := loadAnySystemTTF(t)
	tables, err := parseTTFTables(data)
	if err != nil {
		t.Fatalf("parseTTFTables: %v", err)
	}
	hheaData, ok := tables["hhea"]
	if !ok {
		t.Skip("system font has no hhea table")
	}
	h, err := parseHhea(hheaData)
	if err != nil {
		t.Fatalf("parseHhea on system font: %v", err)
	}
	if h.numberOfHMetrics == 0 {
		t.Error("real font has numberOfHMetrics = 0")
	}
	if h.ascender <= 0 {
		t.Errorf("real font has non-positive ascender %d", h.ascender)
	}
}
