// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "encoding/binary"

// isCIDKeyedCFFv1 reports whether cff carries a CID-keyed CFF v1 font
// — specifically a Top DICT that opens with the ROS operator (12 30,
// per Adobe Tech Note #5176 §9 Table 10). Only CID-keyed CFF v1 is
// safe to embed under PDF's /CIDFontType0 + /CIDFontType0C path; plain
// (name-keyed) CFF needs the /Type1C stream subtype and a non-composite
// font dictionary, which Folio's embedder does not yet build.
//
// Returns false (with no error) for inputs that are too short, malformed,
// CFF font collections (Top DICT INDEX count > 1), or simply not
// CID-keyed. The caller treats any false return as "don't take the CFF
// path" and falls back to the legacy embed semantics.
func isCIDKeyedCFFv1(cff []byte) bool {
	dict, ok := firstTopDICTv1(cff)
	if !ok {
		return false
	}
	return topDICTStartsWithROS(dict)
}

// firstTopDICTv1 extracts the bytes of the first Top DICT in a CFF v1
// blob — Header → Name INDEX → Top DICT INDEX → first object's data.
// Returns (nil, false) on any structural anomaly. The function reads
// fields defensively against the buffer length so a hostile or
// truncated CFF cannot cause an out-of-bounds slice.
func firstTopDICTv1(cff []byte) ([]byte, bool) {
	// Header: major(1), minor(1), hdrSize(1), offSize(1).
	if len(cff) < 4 {
		return nil, false
	}
	if cff[0] != 1 {
		// CFF2 (major=2) or unknown major version. Caller treats
		// CFF2 separately; here we only accept v1.
		return nil, false
	}
	hdrSize := int(cff[2])
	if hdrSize < 4 || hdrSize > len(cff) {
		return nil, false
	}

	// Name INDEX, then Top DICT INDEX. We only need to skip past the
	// Name INDEX; its contents are irrelevant for ROS detection.
	pos := hdrSize
	pos, ok := skipINDEX(cff, pos)
	if !ok {
		return nil, false
	}

	// Top DICT INDEX header: count(uint16), offSize(uint8),
	// offsets[count+1].
	if pos+3 > len(cff) {
		return nil, false
	}
	count := int(binary.BigEndian.Uint16(cff[pos : pos+2]))
	if count != 1 {
		// CFF font collections are spec-legal but not used inside
		// OpenType sfnt wrappers. Reject defensively rather than
		// guessing which Top DICT applies.
		return nil, false
	}
	offSize := int(cff[pos+2])
	if offSize < 1 || offSize > 4 {
		return nil, false
	}
	pos += 3

	// offsets are 1-indexed (first byte after the array is offset 1).
	// We need offsets[0] and offsets[1] to bracket the first object.
	offBytes := (count + 1) * offSize
	if pos+offBytes > len(cff) {
		return nil, false
	}
	off0 := readOffset(cff[pos:], offSize)
	off1 := readOffset(cff[pos+offSize:], offSize)
	if off0 < 1 || off1 < off0 {
		return nil, false
	}
	dataStart := pos + offBytes
	dictStart := dataStart + off0 - 1
	dictEnd := dataStart + off1 - 1
	if dictStart < dataStart || dictEnd > len(cff) || dictEnd < dictStart {
		return nil, false
	}
	return cff[dictStart:dictEnd], true
}

// skipINDEX advances pos past one CFF INDEX (count + offSize + offsets
// + data) and returns the new position. An empty INDEX (count == 0) is
// the 2-byte sequence {0x00 0x00} per spec.
func skipINDEX(buf []byte, pos int) (int, bool) {
	if pos+2 > len(buf) {
		return 0, false
	}
	count := int(binary.BigEndian.Uint16(buf[pos : pos+2]))
	if count == 0 {
		return pos + 2, true
	}
	if pos+3 > len(buf) {
		return 0, false
	}
	offSize := int(buf[pos+2])
	if offSize < 1 || offSize > 4 {
		return 0, false
	}
	offBase := pos + 3
	offBytes := (count + 1) * offSize
	if offBase+offBytes > len(buf) {
		return 0, false
	}
	// Total payload size lives in the last offset entry, minus 1
	// (offsets are 1-indexed).
	lastOff := readOffset(buf[offBase+count*offSize:], offSize)
	if lastOff < 1 {
		return 0, false
	}
	end := offBase + offBytes + lastOff - 1
	if end > len(buf) || end < offBase {
		return 0, false
	}
	return end, true
}

// readOffset reads a 1-, 2-, 3-, or 4-byte big-endian unsigned integer.
// CFF INDEX offsets use this variable-width encoding selected per
// INDEX by its offSize field.
func readOffset(b []byte, n int) int {
	switch n {
	case 1:
		return int(b[0])
	case 2:
		return int(binary.BigEndian.Uint16(b[:2]))
	case 3:
		return int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	case 4:
		return int(binary.BigEndian.Uint32(b[:4]))
	}
	return 0
}

// topDICTStartsWithROS walks a Top DICT's operand/operator stream and
// reports whether the first operator encountered is ROS (12 30). It
// parses the well-defined operand encodings from TN #5176 §4 rather
// than scanning bytes blindly — operand payloads can contain the bytes
// 0x0C and 0x1E and would otherwise yield false positives.
//
// We bail out on the first operator, on operand decoding errors, or
// when the buffer is exhausted. False is the safe answer in every
// failure mode: the caller falls back to the legacy embed path.
func topDICTStartsWithROS(dict []byte) bool {
	pos := 0
	for pos < len(dict) {
		b := dict[pos]
		switch {
		case b <= 21:
			// Operator. 0x0C is the two-byte escape for extended ops.
			if b == 12 {
				if pos+1 >= len(dict) {
					return false
				}
				return dict[pos+1] == 30 // ROS
			}
			// Any other operator means this is not a CID-keyed Top
			// DICT — ROS is required as the first operator.
			return false
		case b == 28:
			pos += 3
		case b == 29:
			pos += 5
		case b == 30:
			// BCD real: nibbles 0..9 are digits, A=., B=E, C=E-,
			// D=reserved, E=minus, F=end. Walk bytes until either
			// nibble is 0xF.
			pos++
			ended := false
			for pos < len(dict) {
				v := dict[pos]
				pos++
				if v&0x0F == 0x0F || v>>4 == 0x0F {
					ended = true
					break
				}
			}
			if !ended {
				return false
			}
		case b >= 32 && b <= 246:
			pos++
		case b >= 247 && b <= 254:
			pos += 2
		default:
			// 22..27, 31, 255 are reserved/unused.
			return false
		}
	}
	return false
}
