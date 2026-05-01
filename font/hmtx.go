// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// parseHmtx decodes the `hmtx` table from raw bytes into per-glyph
// advance widths and left-side bearings.
//
// Spec: ISO/IEC 14496-22 §5.2.4 — Horizontal Metrics table layout.
//
// The first numberOfHMetrics glyphs each have a longHorMetric record:
//   - advanceWidth (uint16)
//   - lsb (int16)
//
// totaling 4 bytes. The remaining (numGlyphs - numberOfHMetrics)
// glyphs share the last advance and contribute only an lsb (int16,
// 2 bytes each). This compaction lets monospaced and CJK fonts skip
// repeating identical advances.
//
// numberOfHMetrics is required to be at least 1 by the spec.
func parseHmtx(data []byte, numberOfHMetrics, numGlyphs int) ([]uint16, []int16, error) {
	if numberOfHMetrics <= 0 {
		return nil, nil, fmt.Errorf("hmtx: numberOfHMetrics must be >= 1, got %d: %w", numberOfHMetrics, ErrCorruptTable)
	}
	if numGlyphs < numberOfHMetrics {
		return nil, nil, fmt.Errorf("hmtx: numGlyphs %d < numberOfHMetrics %d: %w", numGlyphs, numberOfHMetrics, ErrCorruptTable)
	}
	tail := numGlyphs - numberOfHMetrics
	need := uint64(numberOfHMetrics)*4 + uint64(tail)*2
	if uint64(len(data)) < need {
		return nil, nil, fmt.Errorf("hmtx: table truncated (have %d, need %d): %w", len(data), need, ErrTruncated)
	}

	advances := make([]uint16, numGlyphs)
	lsbs := make([]int16, numGlyphs)
	for i := range numberOfHMetrics {
		off := i * 4
		advances[i] = binary.BigEndian.Uint16(data[off : off+2])
		lsbs[i] = int16(binary.BigEndian.Uint16(data[off+2 : off+4]))
	}
	lastAdvance := advances[numberOfHMetrics-1]
	for i := range tail {
		off := numberOfHMetrics*4 + i*2
		gid := numberOfHMetrics + i
		advances[gid] = lastAdvance
		lsbs[gid] = int16(binary.BigEndian.Uint16(data[off : off+2]))
	}
	return advances, lsbs, nil
}
