// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"slices"
	"testing"
)

// --- parseCFFIndex ---

func TestParseCFFIndexEmpty(t *testing.T) {
	// Empty INDEX = two zero bytes for count == 0; no offSize follows.
	raw := []byte{0x00, 0x00}
	idx, err := parseCFFIndex(raw, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx.count != 0 {
		t.Errorf("count = %d, want 0", idx.count)
	}
	if idx.rawEnd != 2 {
		t.Errorf("rawEnd = %d, want 2", idx.rawEnd)
	}
	if idx.Object(0) != nil {
		t.Errorf("Object(0) on empty INDEX must be nil")
	}
}

func TestParseCFFIndexSingleEntry(t *testing.T) {
	// count=1, offSize=1, offsets=[1,5], data="Test"
	raw := []byte{0x00, 0x01, 0x01, 0x01, 0x05, 'T', 'e', 's', 't'}
	idx, err := parseCFFIndex(raw, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx.count != 1 {
		t.Errorf("count = %d, want 1", idx.count)
	}
	if !bytes.Equal(idx.Object(0), []byte("Test")) {
		t.Errorf("Object(0) = %q, want %q", idx.Object(0), "Test")
	}
	if idx.Object(1) != nil {
		t.Errorf("Object(1) out of range should be nil")
	}
	if idx.rawEnd != len(raw) {
		t.Errorf("rawEnd = %d, want %d", idx.rawEnd, len(raw))
	}
}

func TestParseCFFIndexMultiEntryVariousOffSize(t *testing.T) {
	// Three entries with offSize=2 so we exercise the multi-byte path.
	// offsets[0]=1, then sizes 4, 0, 6 → offsets [1,5,5,11].
	objects := [][]byte{[]byte("aaaa"), {}, []byte("ffffff")}
	raw := writeIndex(t, objects, 2)
	idx, err := parseCFFIndex(raw, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx.count != 3 {
		t.Fatalf("count = %d, want 3", idx.count)
	}
	for i, want := range objects {
		got := idx.Object(i)
		if !bytes.Equal(got, want) {
			t.Errorf("Object(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestParseCFFIndexTruncatedHeader(t *testing.T) {
	for _, n := range []int{0, 1} {
		_, err := parseCFFIndex(make([]byte, n), 0)
		if !errors.Is(err, ErrTruncated) {
			t.Errorf("len=%d: err=%v, want ErrTruncated", n, err)
		}
	}
}

func TestParseCFFIndexBadOffSize(t *testing.T) {
	for _, off := range []byte{0, 5, 255} {
		raw := []byte{0x00, 0x01, off, 0x01, 0x05, 'T', 'e', 's', 't'}
		_, err := parseCFFIndex(raw, 0)
		if !errors.Is(err, ErrCorruptTable) {
			t.Errorf("offSize=%d: err=%v, want ErrCorruptTable", off, err)
		}
	}
}

func TestParseCFFIndexFirstOffsetNotOne(t *testing.T) {
	// Same as the single-entry case but offsets[0] = 2.
	raw := []byte{0x00, 0x01, 0x01, 0x02, 0x06, 'X', 'T', 'e', 's', 't'}
	_, err := parseCFFIndex(raw, 0)
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err=%v, want ErrCorruptTable", err)
	}
}

func TestParseCFFIndexNonMonotonic(t *testing.T) {
	// count=2, offsets=[1,5,3] — second offset goes backwards.
	raw := []byte{0x00, 0x02, 0x01, 0x01, 0x05, 0x03, 'T', 'e', 's', 't'}
	_, err := parseCFFIndex(raw, 0)
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err=%v, want ErrCorruptTable", err)
	}
}

func TestParseCFFIndexPayloadTruncated(t *testing.T) {
	// Declared payload size 100 bytes, only 4 actually present.
	raw := []byte{0x00, 0x01, 0x01, 0x01, 101, 'T', 'e', 's', 't'}
	_, err := parseCFFIndex(raw, 0)
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err=%v, want ErrTruncated", err)
	}
}

// writeIndex constructs an INDEX with the supplied object payloads and
// the requested offSize. The helper is the inverse of parseCFFIndex
// and exists so DICT/CFF tests can build fixtures without duplicating
// offset math.
func writeIndex(t *testing.T, objects [][]byte, offSize int) []byte {
	t.Helper()
	if offSize < 1 || offSize > 4 {
		t.Fatalf("writeIndex: offSize %d", offSize)
	}
	count := len(objects)
	if count == 0 {
		return []byte{0x00, 0x00}
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint16(count)); err != nil {
		t.Fatalf("writeIndex: %v", err)
	}
	buf.WriteByte(byte(offSize))
	off := 1
	writeOff := func(v int) {
		b := make([]byte, offSize)
		switch offSize {
		case 1:
			b[0] = byte(v)
		case 2:
			binary.BigEndian.PutUint16(b, uint16(v))
		case 3:
			b[0] = byte(v >> 16)
			b[1] = byte(v >> 8)
			b[2] = byte(v)
		case 4:
			binary.BigEndian.PutUint32(b, uint32(v))
		}
		buf.Write(b)
	}
	writeOff(off)
	for _, o := range objects {
		off += len(o)
		writeOff(off)
	}
	for _, o := range objects {
		buf.Write(o)
	}
	return buf.Bytes()
}

// --- parseCFFDict ---

func TestParseCFFDictIntegerOperands(t *testing.T) {
	// Operand 0 (139), then 'version' operator.
	d, err := parseCFFDict([]byte{139, 0})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(d) != 1 {
		t.Fatalf("entries = %d, want 1", len(d))
	}
	e := d[0]
	if e.operator != cffOpVersion {
		t.Errorf("operator = %d, want %d", e.operator, cffOpVersion)
	}
	if len(e.intOperands) != 1 || e.intOperands[0] != 0 {
		t.Errorf("operands = %v, want [0]", e.intOperands)
	}
	if e.operandStart != 0 || e.operandEnd != 1 {
		t.Errorf("operand range = [%d,%d], want [0,1]", e.operandStart, e.operandEnd)
	}
}

func TestParseCFFDictTwoByteOperator(t *testing.T) {
	// Three zero operands, ROS (12 30).
	d, err := parseCFFDict([]byte{139, 139, 139, 12, 30})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(d) != 1 || d[0].operator != cffOp2ROS {
		t.Fatalf("expected single ROS entry; got %+v", d)
	}
	if len(d[0].intOperands) != 3 {
		t.Fatalf("operand count = %d, want 3", len(d[0].intOperands))
	}
	for i, v := range d[0].intOperands {
		if v != 0 {
			t.Errorf("operand[%d] = %d, want 0", i, v)
		}
	}
	if d[0].operandEnd != 3 {
		t.Errorf("operandEnd = %d, want 3", d[0].operandEnd)
	}
	// Per-operand spans: each operand is one byte (139 → 0).
	wantSpans := [][2]int{{0, 1}, {1, 2}, {2, 3}}
	if len(d[0].operandSpans) != len(wantSpans) {
		t.Fatalf("spans len = %d, want %d", len(d[0].operandSpans), len(wantSpans))
	}
	for i, want := range wantSpans {
		if d[0].operandSpans[i] != want {
			t.Errorf("operandSpans[%d] = %v, want %v", i, d[0].operandSpans[i], want)
		}
	}
}

func TestParseCFFDictMultipleEntries(t *testing.T) {
	// Two entries: charset @ offset 50, CharStrings @ offset 100.
	// Encoded: 247 (=108), op 15 charset, 247 (=108)... but 247
	// is two-byte: 247 N → 0..1131. Let me use simple ints.
	// 50 = 50+139 = 189 (1 byte). 100 = 100+139 = 239 (1 byte).
	d, err := parseCFFDict([]byte{189, 15, 239, 17})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(d) != 2 {
		t.Fatalf("entries = %d, want 2", len(d))
	}
	if d[0].operator != cffOpCharset || d[0].intOperands[0] != 50 {
		t.Errorf("entry 0 = %+v", d[0])
	}
	if d[1].operator != cffOpCharStrings || d[1].intOperands[0] != 100 {
		t.Errorf("entry 1 = %+v", d[1])
	}
	// operandStart/End for entry 1: starts right after entry 0's
	// operator, which is at byte 1. So operands start at byte 2 and
	// the operator at byte 3.
	if d[1].operandStart != 2 || d[1].operandEnd != 3 {
		t.Errorf("entry 1 operand range = [%d,%d]", d[1].operandStart, d[1].operandEnd)
	}
}

func TestParseCFFDictShortIntEncoding(t *testing.T) {
	// CFF v1 §4 Table 3: byte 28 is the shortint prefix — the next
	// two bytes encode a signed 16-bit big-endian integer. 1500 is
	// 0x05DC.
	cases := []struct {
		bytes []byte
		want  int64
	}{
		{[]byte{28, 0x05, 0xDC, 0}, 1500},
		{[]byte{28, 0x7F, 0xFF, 0}, 32767},        // INT16 max
		{[]byte{28, 0x80, 0x00, 0}, -32768},       // INT16 min
		{[]byte{28, 0xFF, 0xFF, 0}, -1},           // sign extension
	}
	for _, tc := range cases {
		d, err := parseCFFDict(tc.bytes)
		if err != nil {
			t.Errorf("%v: err = %v", tc.bytes, err)
			continue
		}
		if d[0].intOperands[0] != tc.want {
			t.Errorf("%v: operand = %d, want %d", tc.bytes, d[0].intOperands[0], tc.want)
		}
		// shortint occupies exactly 3 bytes.
		if d[0].operandSpans[0] != [2]int{0, 3} {
			t.Errorf("%v: span = %v, want [0,3]", tc.bytes, d[0].operandSpans[0])
		}
	}
}

func TestParseCFFDictTwoByteIntPositive(t *testing.T) {
	// CFF v1 §4 Table 3: bytes 247..250 are the two-byte int prefix
	// for positive values 108..1131. Encoding: (b0-247)*256 + b1 + 108.
	cases := []struct {
		bytes []byte
		want  int64
	}{
		{[]byte{247, 0, 0}, 108},          // 247: minimum
		{[]byte{247, 255, 0}, 363},        // 247 with max N
		{[]byte{250, 255, 0}, 1131},       // 250: maximum
	}
	for _, tc := range cases {
		d, err := parseCFFDict(tc.bytes)
		if err != nil {
			t.Errorf("%v: err = %v", tc.bytes, err)
			continue
		}
		if d[0].intOperands[0] != tc.want {
			t.Errorf("%v: operand = %d, want %d", tc.bytes, d[0].intOperands[0], tc.want)
		}
		if d[0].operandSpans[0] != [2]int{0, 2} {
			t.Errorf("%v: span = %v, want [0,2]", tc.bytes, d[0].operandSpans[0])
		}
	}
}

func TestParseCFFDictTwoByteIntNegative(t *testing.T) {
	// Bytes 251..254: negative two-byte int. Encoding:
	// -(b0-251)*256 - b1 - 108. Range -1131..-108.
	cases := []struct {
		bytes []byte
		want  int64
	}{
		{[]byte{251, 0, 0}, -108},
		{[]byte{254, 255, 0}, -1131},
	}
	for _, tc := range cases {
		d, err := parseCFFDict(tc.bytes)
		if err != nil {
			t.Errorf("%v: err = %v", tc.bytes, err)
			continue
		}
		if d[0].intOperands[0] != tc.want {
			t.Errorf("%v: operand = %d, want %d", tc.bytes, d[0].intOperands[0], tc.want)
		}
	}
}

func TestParseCFFDictLongIntBoundaries(t *testing.T) {
	cases := []struct {
		want int32
	}{
		{0},
		{1},
		{-1},
		{1 << 30},
		{-(1 << 30)},
		{0x7FFFFFFF}, // INT32 max
		{-1 << 31},   // INT32 min
	}
	for _, tc := range cases {
		buf := []byte{29, 0, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(buf[1:5], uint32(tc.want))
		d, err := parseCFFDict(buf)
		if err != nil {
			t.Errorf("longint %d: err = %v", tc.want, err)
			continue
		}
		if d[0].intOperands[0] != int64(tc.want) {
			t.Errorf("longint %d: got %d", tc.want, d[0].intOperands[0])
		}
		if d[0].operandSpans[0] != [2]int{0, 5} {
			t.Errorf("longint %d: span = %v, want [0,5]", tc.want, d[0].operandSpans[0])
		}
	}
}

func TestParseCFFDictMixedIntRealOperands(t *testing.T) {
	// Pattern: int, real, int, operator. The intOperands slot for the
	// real position must be zero, realIndices must point at it, and
	// operandSpans must bracket each operand precisely.
	dict := []byte{
		139,        // int 0
		30, 0x1F,   // real, end nibble F (digit 1 + end)
		140,        // int 1
		0,          // operator: version
	}
	d, err := parseCFFDict(dict)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(d) != 1 {
		t.Fatalf("entries = %d", len(d))
	}
	e := d[0]
	if len(e.intOperands) != 3 || e.intOperands[0] != 0 || e.intOperands[2] != 1 {
		t.Errorf("intOperands = %v, want [0, 0, 1]", e.intOperands)
	}
	if len(e.realIndices) != 1 || e.realIndices[0] != 1 {
		t.Errorf("realIndices = %v, want [1]", e.realIndices)
	}
	wantSpans := [][2]int{{0, 1}, {1, 3}, {3, 4}}
	for i, want := range wantSpans {
		if e.operandSpans[i] != want {
			t.Errorf("operandSpans[%d] = %v, want %v", i, e.operandSpans[i], want)
		}
	}
}

func TestParseCFFDictFourByteIntEncoding(t *testing.T) {
	// 100000 encoded as longint: 29 followed by int32 BE.
	want := int64(100000)
	buf := []byte{29, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(buf[1:5], uint32(want))
	d, err := parseCFFDict(buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d[0].intOperands[0] != want {
		t.Errorf("operand = %d, want %d", d[0].intOperands[0], want)
	}
}

func TestParseCFFDictNegativeOperand(t *testing.T) {
	// 251 0 → -108. Followed by 'version'.
	d, err := parseCFFDict([]byte{251, 0, 0})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d[0].intOperands[0] != -108 {
		t.Errorf("operand = %d, want -108", d[0].intOperands[0])
	}
}

func TestParseCFFDictBCDReal(t *testing.T) {
	// BCD real: 30 (prefix), 0x12, 0xFF — digit "12" then end nibble F.
	// Followed by 'version'.
	d, err := parseCFFDict([]byte{30, 0x12, 0xFF, 0})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(d) != 1 {
		t.Fatalf("entries = %d", len(d))
	}
	if len(d[0].intOperands) != 1 || d[0].intOperands[0] != 0 {
		t.Errorf("intOperands = %v, want [0]", d[0].intOperands)
	}
	if len(d[0].realIndices) != 1 || d[0].realIndices[0] != 0 {
		t.Errorf("realIndices = %v, want [0]", d[0].realIndices)
	}
	// BCD operand occupies bytes [0, 3): prefix 30 + 0x12 + 0xFF.
	if len(d[0].operandSpans) != 1 || d[0].operandSpans[0] != [2]int{0, 3} {
		t.Errorf("operandSpans = %v, want [[0,3]]", d[0].operandSpans)
	}
}

func TestParseCFFDictTwoByteOperatorTruncated(t *testing.T) {
	// A lone byte 12 with no second byte means a 2-byte operator
	// escape with the operator code missing.
	_, err := parseCFFDict([]byte{12})
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want ErrTruncated", err)
	}
}

func TestParseCFFDictReservedByteFails(t *testing.T) {
	for _, b := range []byte{22, 23, 24, 25, 26, 27, 31, 255} {
		_, err := parseCFFDict([]byte{b})
		if !errors.Is(err, ErrCorruptTable) {
			t.Errorf("byte 0x%02X: err=%v, want ErrCorruptTable", b, err)
		}
	}
}

func TestParseCFFDictTruncatedOperand(t *testing.T) {
	// shortint with only one trailing byte.
	_, err := parseCFFDict([]byte{28, 0x05})
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want ErrTruncated", err)
	}
}

// --- computeCharsetSize ---

func TestComputeCharsetSizeFormat0(t *testing.T) {
	// numGlyphs=4 → 3 SIDs of 2 bytes each → 1 + 6 = 7 bytes.
	raw := append([]byte{0x00}, bytes.Repeat([]byte{0xAA}, 6)...)
	size, err := computeCharsetSize(raw, 0, 4)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if size != 7 {
		t.Errorf("size = %d, want 7", size)
	}
}

func TestComputeCharsetSizeFormat1(t *testing.T) {
	// Format 1: range (firstSID uint16, nLeft uint8). Covering 3
	// non-notdef glyphs in one range: nLeft = 2. Total 1 + 3 = 4 bytes.
	raw := []byte{0x01, 0x00, 0x01, 0x02}
	size, err := computeCharsetSize(raw, 0, 4)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if size != 4 {
		t.Errorf("size = %d, want 4", size)
	}
}

func TestComputeCharsetSizeFormat2(t *testing.T) {
	// Format 2: range (firstSID uint16, nLeft uint16). One range
	// covering 3 glyphs.
	raw := []byte{0x02, 0x00, 0x01, 0x00, 0x02}
	size, err := computeCharsetSize(raw, 0, 4)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if size != 5 {
		t.Errorf("size = %d, want 5", size)
	}
}

func TestComputeCharsetSizeUnknownFormat(t *testing.T) {
	_, err := computeCharsetSize([]byte{0x77, 0x00}, 0, 4)
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want ErrCorruptTable", err)
	}
}

func TestComputeCharsetSizeFormat1MultiRange(t *testing.T) {
	// Three ranges of 2 glyphs each (nLeft=1 means 2 glyphs covered),
	// covering 6 of the 7 non-notdef glyphs.
	raw := []byte{
		0x01,
		0x00, 0x01, 0x01, // range 1: first=1, nLeft=1
		0x00, 0x03, 0x01, // range 2: first=3, nLeft=1
		0x00, 0x05, 0x01, // range 3: first=5, nLeft=1
	}
	size, err := computeCharsetSize(raw, 0, 7)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if size != 10 {
		t.Errorf("size = %d, want 10", size)
	}
}

func TestComputeCharsetSizeFormat2MultiRange(t *testing.T) {
	// Two ranges. Format 2 uses uint16 nLeft so we can cover many
	// glyphs per range.
	raw := []byte{
		0x02,
		0x00, 0x01, 0x00, 0x02, // range 1: first=1, nLeft=2 (3 glyphs)
		0x00, 0x05, 0x00, 0x02, // range 2: first=5, nLeft=2 (3 glyphs)
	}
	size, err := computeCharsetSize(raw, 0, 7)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if size != 9 {
		t.Errorf("size = %d, want 9", size)
	}
}

func TestComputeCharsetSizeFormat1OverCoverage(t *testing.T) {
	// One range claiming to cover 6 glyphs but numGlyphs-1 = 3.
	raw := []byte{0x01, 0x00, 0x01, 0x05}
	_, err := computeCharsetSize(raw, 0, 4)
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want ErrCorruptTable", err)
	}
}

func TestComputeCharsetSizeFormat2OverCoverage(t *testing.T) {
	raw := []byte{0x02, 0x00, 0x01, 0x00, 0x05}
	_, err := computeCharsetSize(raw, 0, 4)
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want ErrCorruptTable", err)
	}
}

// --- computeFDSelectSize ---

func TestComputeFDSelectSizeFormat0(t *testing.T) {
	raw := append([]byte{0x00}, bytes.Repeat([]byte{0x00}, 10)...)
	size, err := computeFDSelectSize(raw, 0, 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if size != 11 {
		t.Errorf("size = %d, want 11", size)
	}
}

func TestComputeFDSelectSizeFormat3(t *testing.T) {
	// Two ranges then sentinel: 1 + 2 + 2*3 + 2 = 11 bytes.
	raw := []byte{
		0x03, 0x00, 0x02,
		0x00, 0x00, 0x00, // range 1: first=0, fd=0
		0x00, 0x05, 0x01, // range 2: first=5, fd=1
		0x00, 0x0A, // sentinel = numGlyphs
	}
	size, err := computeFDSelectSize(raw, 0, 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if size != 11 {
		t.Errorf("size = %d, want 11", size)
	}
}

func TestComputeFDSelectSizeFormat3SentinelMismatch(t *testing.T) {
	raw := []byte{
		0x03, 0x00, 0x02,
		0x00, 0x00, 0x00,
		0x00, 0x05, 0x01,
		0x00, 0x0B, // sentinel = 11, but numGlyphs = 10
	}
	_, err := computeFDSelectSize(raw, 0, 10)
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want ErrCorruptTable", err)
	}
}

// --- parseCFF integration ---

// TestParseCFFRealFontSmoke ensures parseCFF accepts a real CID-keyed
// font end-to-end. The test skips when no CID-keyed CFF is available,
// which is acceptable for now — the unit tests above cover the
// structural code paths deterministically. Phase 3 will land a
// synthetic CID-keyed CFF builder for full CI coverage.
func TestParseCFFRealFontSmoke(t *testing.T) {
	face := loadTestCFFFace(t)
	cf := face.(cffFace)
	cff, err := parseCFF(cf.CFFData())
	if err != nil {
		t.Fatalf("parseCFF: %v", err)
	}
	if cff.numGlyphs <= 0 {
		t.Errorf("numGlyphs = %d", cff.numGlyphs)
	}
	if len(cff.fds) == 0 {
		t.Errorf("no FDs parsed")
	}
	// Every FD must have non-nil font dict bytes and a non-empty
	// private dict byte range; otherwise Phase 3 has nothing to
	// rewrite.
	for i, fd := range cff.fds {
		if len(fd.fontDictBytes) == 0 {
			t.Errorf("fd[%d] empty font dict", i)
		}
		if len(fd.privateBytes) == 0 {
			t.Errorf("fd[%d] empty private dict", i)
		}
	}
	// CharStrings INDEX count must equal numGlyphs.
	if cff.charStringsIndex.count != cff.numGlyphs {
		t.Errorf("charstrings count %d != numGlyphs %d",
			cff.charStringsIndex.count, cff.numGlyphs)
	}
	// charsetSize and fdSelectSize must be positive and stay within
	// raw bounds.
	if cff.charsetSize <= 0 || cff.charsetOffset+cff.charsetSize > len(cff.raw) {
		t.Errorf("charset offset+size out of range: off=%d size=%d", cff.charsetOffset, cff.charsetSize)
	}
	if cff.fdSelectSize <= 0 || cff.fdSelectOffset+cff.fdSelectSize > len(cff.raw) {
		t.Errorf("fdselect offset+size out of range: off=%d size=%d", cff.fdSelectOffset, cff.fdSelectSize)
	}
}

func TestParseCFFRejectsNonCIDKeyed(t *testing.T) {
	path := testNonCIDKeyedCFFFontPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	face, err := ParseFont(data)
	if err != nil {
		t.Fatalf("ParseFont: %v", err)
	}
	cf := face.(cffFace)
	_, err = parseCFF(cf.CFFData())
	if err == nil {
		t.Fatal("expected parseCFF to reject name-keyed CFF")
	}
	if !errors.Is(err, ErrCorruptTable) && !errors.Is(err, ErrUnknownFormat) {
		t.Errorf("err = %v, want wrap of ErrCorruptTable or ErrUnknownFormat", err)
	}
}

func TestParseCFFRejectsCFF2(t *testing.T) {
	// Build a minimal CFF v1 then flip the major to 2. parseCFF
	// should reject before walking further.
	cff := buildSyntheticCFFv1([]byte{139, 139, 139, 12, 30})
	cff[0] = 2
	_, err := parseCFF(cff)
	if !errors.Is(err, ErrUnknownFormat) {
		t.Errorf("err = %v, want ErrUnknownFormat", err)
	}
}

func TestParseCFFRejectsFontCollection(t *testing.T) {
	dict := []byte{139, 139, 139, 12, 30}
	cff := buildSyntheticCFFCollection(t, dict, dict)
	_, err := parseCFF(cff)
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want ErrCorruptTable", err)
	}
}

// TestParseCFFSyntheticCIDKeyed validates the parser against a
// deterministic CID-keyed CFF that doesn't depend on a system font.
// Verifies every read structural field rather than just "non-nil"
// — a parser regression that yields, say, half the FDs or the wrong
// CharStrings count would slip through a weaker check.
func TestParseCFFSyntheticCIDKeyed(t *testing.T) {
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{numGlyphs: 7, fdCount: 1})
	parsed, err := parseCFF(cff)
	if err != nil {
		t.Fatalf("parseCFF: %v", err)
	}
	if parsed.numGlyphs != 7 {
		t.Errorf("numGlyphs = %d, want 7", parsed.numGlyphs)
	}
	if parsed.charStringsIndex.count != 7 {
		t.Errorf("charstrings count = %d, want 7", parsed.charStringsIndex.count)
	}
	if len(parsed.fds) != 1 {
		t.Errorf("fd count = %d, want 1", len(parsed.fds))
	}
	if parsed.rosRegistry != 391 || parsed.rosOrdering != 392 || parsed.rosSupplement != 0 {
		t.Errorf("ROS triplet = (%d, %d, %d), want (391, 392, 0)",
			parsed.rosRegistry, parsed.rosOrdering, parsed.rosSupplement)
	}
	if parsed.cidCount != 7 {
		t.Errorf("CIDCount = %d, want 7", parsed.cidCount)
	}
	// Each charstring is a single endchar byte in the synthetic blob.
	for i := range parsed.numGlyphs {
		got := parsed.charStringsIndex.Object(i)
		if len(got) != 1 || got[0] != 0x0E {
			t.Errorf("charstring %d = %v, want [0x0E]", i, got)
		}
	}
	// Private DICT must locate the Local Subr INDEX immediately after.
	fd := parsed.fds[0]
	if fd.localSubrs == nil {
		t.Fatal("expected non-nil local subrs")
	}
	if fd.localSubrs.count != 0 {
		t.Errorf("local subrs count = %d, want 0", fd.localSubrs.count)
	}
}

func TestParseCFFSyntheticMultiFD(t *testing.T) {
	const fdCount = 5
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{numGlyphs: 12, fdCount: fdCount})
	parsed, err := parseCFF(cff)
	if err != nil {
		t.Fatalf("parseCFF: %v", err)
	}
	if len(parsed.fds) != fdCount {
		t.Fatalf("fd count = %d, want %d", len(parsed.fds), fdCount)
	}
	for i, fd := range parsed.fds {
		if len(fd.fontDictBytes) == 0 {
			t.Errorf("fd[%d] empty font dict", i)
		}
		if len(fd.privateBytes) != 18 {
			t.Errorf("fd[%d] private size = %d, want 18", i, len(fd.privateBytes))
		}
		if fd.localSubrs == nil || fd.localSubrs.count != 0 {
			t.Errorf("fd[%d] local subrs missing or non-empty", i)
		}
	}
}

// TestParseCFFRejectionPaths bundles the structural-failure cases:
// missing required Top DICT operator, missing per-FD Private, broken
// FD private pointer. Each was previously only exercised through
// real-font smoke tests that skip on CI.
func TestParseCFFRejectionPaths(t *testing.T) {
	cases := []struct {
		name string
		opts syntheticCFFOptions
		want error
	}{
		{"missing ROS", syntheticCFFOptions{numGlyphs: 3, fdCount: 1, skipROS: true}, ErrCorruptTable},
		{"missing CharStrings", syntheticCFFOptions{numGlyphs: 3, fdCount: 1, skipCharStrings: true}, ErrCorruptTable},
		{"missing FDArray", syntheticCFFOptions{numGlyphs: 3, fdCount: 1, skipFDArray: true}, ErrCorruptTable},
		{"missing FDSelect", syntheticCFFOptions{numGlyphs: 3, fdCount: 1, skipFDSelect: true}, ErrCorruptTable},
		{"FD missing Private op", syntheticCFFOptions{numGlyphs: 3, fdCount: 1, skipFDPrivate: true}, ErrCorruptTable},
		{"FD Private out of range", syntheticCFFOptions{numGlyphs: 3, fdCount: 1, brokenFDPrivate: true}, ErrCorruptTable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cff := buildSyntheticCIDKeyedCFF(t, tc.opts)
			_, err := parseCFF(cff)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want wrap of %v", err, tc.want)
			}
		})
	}
}

// TestParseCFFEmptyCharStrings verifies parseCFF rejects a CFF with a
// CharStrings INDEX of count 0. The synthetic builder doesn't expose
// this case directly (it requires numGlyphs >= 1), so we patch the
// CharStrings INDEX header byte by byte.
func TestParseCFFEmptyCharStrings(t *testing.T) {
	cff := buildSyntheticCIDKeyedCFF(t, syntheticCFFOptions{numGlyphs: 1, fdCount: 1})
	// Find the CharStrings INDEX by re-parsing and editing the count
	// uint16 at its rawStart in-place. The original is `00 01`; flip
	// to `00 00` and shorten following sections accordingly. The
	// simpler test: hand-build a CharStrings INDEX with count=0 inside
	// a CFF that otherwise parses, by stitching synthetic pieces.
	//
	// We take the pragmatic route: re-use the parser to locate the
	// offset, overwrite the count, and trust that downstream parsing
	// will fail at the right step.
	parsed, err := parseCFF(cff)
	if err != nil {
		t.Fatalf("baseline parse failed: %v", err)
	}
	pos := parsed.charStringsIndex.rawStart
	// Overwrite the INDEX header in a copy.
	tampered := slices.Clone(cff)
	tampered[pos] = 0x00
	tampered[pos+1] = 0x00
	_, err = parseCFF(tampered)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want ErrCorruptTable", err)
	}
}
