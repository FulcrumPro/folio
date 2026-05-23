// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// hhea is the parsed contents of the TrueType / OpenType `hhea` table,
// limited to the fields Folio's metric path consults.
//
// Spec: ISO/IEC 14496-22 §5.2.3 — Horizontal Header table layout.
type hhea struct {
	ascender         int16
	descender        int16
	lineGap          int16
	numberOfHMetrics uint16
}

// parseHhea decodes the `hhea` table from raw bytes.
//
// Field offsets (from §5.2.3):
//   - majorVersion (uint16)   at 0
//   - minorVersion (uint16)   at 2
//   - ascender (FWord int16)  at 4
//   - descender (FWord int16) at 6
//   - lineGap (FWord int16)   at 8
//   - advanceWidthMax (UFWord uint16) at 10
//   - minLeftSideBearing  (int16) at 12
//   - minRightSideBearing (int16) at 14
//   - xMaxExtent  (int16) at 16
//   - caretSlopeRise   (int16) at 18
//   - caretSlopeRun    (int16) at 20
//   - caretOffset      (int16) at 22
//   - reserved x4              at 24..32
//   - metricDataFormat (int16) at 32
//   - numberOfHMetrics (uint16) at 34
//
// Minimum length is 36 bytes.
func parseHhea(data []byte) (hhea, error) {
	if uint64(len(data)) < 36 {
		return hhea{}, fmt.Errorf("font: hhea: table truncated (%d < 36 bytes): %w", len(data), ErrTruncated)
	}
	return hhea{
		ascender:         int16(binary.BigEndian.Uint16(data[4:6])),
		descender:        int16(binary.BigEndian.Uint16(data[6:8])),
		lineGap:          int16(binary.BigEndian.Uint16(data[8:10])),
		numberOfHMetrics: binary.BigEndian.Uint16(data[34:36]),
	}, nil
}
