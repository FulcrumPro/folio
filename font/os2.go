// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// os2 is the parsed contents of the TrueType / OpenType `OS/2` table,
// limited to the fields Folio's metric path consults.
//
// Spec: ISO/IEC 14496-22 §5.2.7 — OS/2 and Windows Metrics table
// layout. Field offsets here cite the v0+ table; sCapHeight and
// sxHeight require version >= 2.
type os2 struct {
	version        uint16
	sTypoAscender  int16
	sTypoDescender int16
	sTypoLineGap   int16
	usWinAscent    uint16
	usWinDescent   uint16
	fsSelection    uint16
	sxHeight       int16 // 0 when version < 2
	sCapHeight     int16 // 0 when version < 2
}

// useTypoMetrics reports the OS/2 fsSelection USE_TYPO_METRICS bit
// (bit 7, value 0x80). When set, ascent/descent should be derived
// from sTypoAscender/sTypoDescender; otherwise the Win metrics
// (with negative-signed descent in the Windows convention) take
// precedence. Folio uses the typo metrics unconditionally where
// available because they more closely match a font's intended
// line-box geometry; the bit is exposed for callers that care.
func (o *os2) useTypoMetrics() bool {
	return o.fsSelection&0x80 != 0
}

// parseOS2 decodes the `OS/2` table from raw bytes. The table's
// length depends on its version (78 bytes for v0, 96 for v2/3,
// 100 for v5). We only consult fields up to offset 90 (sCapHeight),
// guarded by per-field length checks so older fonts with truncated
// OS/2 sections still parse.
//
// Field offsets (§5.2.7, version 4 layout — earlier versions are a
// strict prefix):
//   - version (uint16)         at  0
//   - xAvgCharWidth (int16)    at  2
//   - usWeightClass (uint16)   at  4
//   - usWidthClass (uint16)    at  6
//   - fsType (uint16)          at  8
//   - ySubscriptXSize..yStrikeoutPosition  at 10..28
//   - sFamilyClass (int16)     at 30
//   - panose[10]               at 32..42
//   - ulUnicodeRange1..4       at 42..58
//   - achVendID[4]             at 58..62
//   - fsSelection (uint16)     at 62
//   - usFirstCharIndex (uint16) at 64
//   - usLastCharIndex  (uint16) at 66
//   - sTypoAscender   (int16)  at 68
//   - sTypoDescender  (int16)  at 70
//   - sTypoLineGap    (int16)  at 72
//   - usWinAscent (uint16)     at 74
//   - usWinDescent (uint16)    at 76
//   - ulCodePageRange1, 2      at 78..86 (v1+)
//   - sxHeight (int16)         at 86 (v2+)
//   - sCapHeight (int16)       at 88 (v2+)
//   - usDefaultChar..usMaxContext  at 90..96 (v2+)
//
// Version 0 is 78 bytes; versions 1 and up extend the trailing fields.
func parseOS2(data []byte) (*os2, error) {
	if uint64(len(data)) < 78 {
		return nil, fmt.Errorf("OS/2: table truncated (%d < 78 bytes): %w", len(data), ErrTruncated)
	}
	o := &os2{
		version:        binary.BigEndian.Uint16(data[0:2]),
		fsSelection:    binary.BigEndian.Uint16(data[62:64]),
		sTypoAscender:  int16(binary.BigEndian.Uint16(data[68:70])),
		sTypoDescender: int16(binary.BigEndian.Uint16(data[70:72])),
		sTypoLineGap:   int16(binary.BigEndian.Uint16(data[72:74])),
		usWinAscent:    binary.BigEndian.Uint16(data[74:76]),
		usWinDescent:   binary.BigEndian.Uint16(data[76:78]),
	}
	if o.version >= 2 && uint64(len(data)) >= 90 {
		o.sxHeight = int16(binary.BigEndian.Uint16(data[86:88]))
		o.sCapHeight = int16(binary.BigEndian.Uint16(data[88:90]))
	}
	return o, nil
}
