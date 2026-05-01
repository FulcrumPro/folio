// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// maxp is the parsed contents of the TrueType / OpenType `maxp` table,
// limited to the field Folio's metric path consults: numGlyphs.
//
// Spec: ISO/IEC 14496-22 §5.2.6 — Maximum Profile table layout.
type maxp struct {
	numGlyphs uint16
}

// parseMaxp decodes the `maxp` table from raw bytes. Both v0.5 (CFF
// fonts) and v1.0 (TrueType outlines) are accepted; only the version
// fixed at offset 0 and numGlyphs at offset 4 are read here.
//
// Field offsets (from §5.2.6):
//   - version (Fixed uint32) at 0 — 0x00005000 (0.5) or 0x00010000 (1.0)
//   - numGlyphs (uint16)     at 4
//
// Minimum length is 6 bytes.
func parseMaxp(data []byte) (maxp, error) {
	if uint64(len(data)) < 6 {
		return maxp{}, fmt.Errorf("maxp: table truncated (%d < 6 bytes): %w", len(data), ErrTruncated)
	}
	n := binary.BigEndian.Uint16(data[4:6])
	if n == 0 {
		return maxp{}, fmt.Errorf("maxp: numGlyphs is zero: %w", ErrCorruptTable)
	}
	return maxp{numGlyphs: n}, nil
}
