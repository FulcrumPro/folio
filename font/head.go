// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// head is the parsed contents of the TrueType / OpenType `head` table,
// limited to the fields Folio's metric path consults.
//
// Spec: ISO/IEC 14496-22 §5.2.2 — Font Header table layout.
type head struct {
	unitsPerEm       uint16
	xMin, yMin       int16
	xMax, yMax       int16
	indexToLocFormat int16
}

// parseHead decodes the `head` table from raw bytes. The caller passes
// only the table's bytes (already extracted from the table directory).
//
// Field offsets (from §5.2.2):
//   - majorVersion (uint16) at 0
//   - minorVersion (uint16) at 2
//   - fontRevision (Fixed)  at 4
//   - checkSumAdjustment    at 8
//   - magicNumber (uint32)  at 12 — 0x5F0F3CF5
//   - flags (uint16)        at 16
//   - unitsPerEm (uint16)   at 18
//   - created   (LONGDATETIME) at 20
//   - modified  (LONGDATETIME) at 28
//   - xMin (int16)          at 36
//   - yMin (int16)          at 38
//   - xMax (int16)          at 40
//   - yMax (int16)          at 42
//   - macStyle (uint16)     at 44
//   - lowestRecPPEM (uint16) at 46
//   - fontDirectionHint (int16) at 48
//   - indexToLocFormat (int16) at 50
//   - glyphDataFormat  (int16) at 52
//
// The minimum length is 54 bytes.
func parseHead(data []byte) (head, error) {
	if uint64(len(data)) < 54 {
		return head{}, fmt.Errorf("head: table truncated (%d < 54 bytes): %w", len(data), ErrTruncated)
	}
	if magic := binary.BigEndian.Uint32(data[12:16]); magic != 0x5F0F3CF5 {
		return head{}, fmt.Errorf("head: magicNumber 0x%08X != 0x5F0F3CF5: %w", magic, ErrCorruptTable)
	}
	upem := binary.BigEndian.Uint16(data[18:20])
	if upem == 0 {
		return head{}, fmt.Errorf("head: unitsPerEm is zero: %w", ErrCorruptTable)
	}
	return head{
		unitsPerEm:       upem,
		xMin:             int16(binary.BigEndian.Uint16(data[36:38])),
		yMin:             int16(binary.BigEndian.Uint16(data[38:40])),
		xMax:             int16(binary.BigEndian.Uint16(data[40:42])),
		yMax:             int16(binary.BigEndian.Uint16(data[42:44])),
		indexToLocFormat: int16(binary.BigEndian.Uint16(data[50:52])),
	}, nil
}
