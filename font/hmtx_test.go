// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"errors"
	"testing"
)

// buildHmtxTable assembles a synthetic hmtx with the given longHorMetric
// records followed by trailing LSB-only entries that share the last
// advance in `metrics`.
func buildHmtxTable(metrics []struct{ adv, lsb int16 }, trailingLSBs []int16) []byte {
	buf := make([]byte, len(metrics)*4+len(trailingLSBs)*2)
	for i, m := range metrics {
		off := i * 4
		binary.BigEndian.PutUint16(buf[off:], uint16(m.adv))
		binary.BigEndian.PutUint16(buf[off+2:], uint16(m.lsb))
	}
	for i, lsb := range trailingLSBs {
		off := len(metrics)*4 + i*2
		binary.BigEndian.PutUint16(buf[off:], uint16(lsb))
	}
	return buf
}

func TestParseHmtxFullLongMetrics(t *testing.T) {
	metrics := []struct{ adv, lsb int16 }{
		{adv: 500, lsb: 50},
		{adv: 600, lsb: -10},
		{adv: 700, lsb: 0},
	}
	data := buildHmtxTable(metrics, nil)
	advances, lsbs, err := parseHmtx(data, 3, 3)
	if err != nil {
		t.Fatalf("parseHmtx: %v", err)
	}
	if got := []uint16{advances[0], advances[1], advances[2]}; got[0] != 500 || got[1] != 600 || got[2] != 700 {
		t.Errorf("advances = %v, want [500 600 700]", got)
	}
	if got := []int16{lsbs[0], lsbs[1], lsbs[2]}; got[0] != 50 || got[1] != -10 || got[2] != 0 {
		t.Errorf("lsbs = %v, want [50 -10 0]", got)
	}
}

func TestParseHmtxTrailingShortMetrics(t *testing.T) {
	// numGlyphs=5, numberOfHMetrics=2; glyphs 2..4 share advance=600.
	metrics := []struct{ adv, lsb int16 }{
		{adv: 500, lsb: 1},
		{adv: 600, lsb: 2},
	}
	trailing := []int16{30, 40, 50}
	data := buildHmtxTable(metrics, trailing)
	advances, lsbs, err := parseHmtx(data, 2, 5)
	if err != nil {
		t.Fatalf("parseHmtx: %v", err)
	}
	want := []uint16{500, 600, 600, 600, 600}
	for i, w := range want {
		if advances[i] != w {
			t.Errorf("advances[%d] = %d, want %d", i, advances[i], w)
		}
	}
	wantLSB := []int16{1, 2, 30, 40, 50}
	for i, w := range wantLSB {
		if lsbs[i] != w {
			t.Errorf("lsbs[%d] = %d, want %d", i, lsbs[i], w)
		}
	}
}

func TestParseHmtxTruncated(t *testing.T) {
	data := []byte{0, 0, 0, 0} // one record, claim two
	_, _, err := parseHmtx(data, 2, 2)
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want errors.Is ErrTruncated", err)
	}
}

func TestParseHmtxBadCounts(t *testing.T) {
	if _, _, err := parseHmtx(nil, 0, 1); !errors.Is(err, ErrCorruptTable) {
		t.Errorf("numH=0: err = %v, want errors.Is ErrCorruptTable", err)
	}
	if _, _, err := parseHmtx(nil, 5, 3); !errors.Is(err, ErrCorruptTable) {
		t.Errorf("numH > numGlyphs: err = %v, want errors.Is ErrCorruptTable", err)
	}
}

func TestParseHmtxOnRealSystemFont(t *testing.T) {
	data := loadAnySystemTTF(t)
	tables, err := parseTTFTables(data)
	if err != nil {
		t.Fatalf("parseTTFTables: %v", err)
	}
	maxp, _ := parseMaxp(tables["maxp"])
	hhea, _ := parseHhea(tables["hhea"])
	hmtx, ok := tables["hmtx"]
	if !ok {
		t.Skip("system font has no hmtx table")
	}
	advances, lsbs, err := parseHmtx(hmtx, int(hhea.numberOfHMetrics), int(maxp.numGlyphs))
	if err != nil {
		t.Fatalf("parseHmtx: %v", err)
	}
	if len(advances) != int(maxp.numGlyphs) || len(lsbs) != int(maxp.numGlyphs) {
		t.Errorf("output sizes = %d/%d, want %d each", len(advances), len(lsbs), maxp.numGlyphs)
	}
}
