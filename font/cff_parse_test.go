// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
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
		t.Errorf("operand count = %d, want 3", len(d[0].intOperands))
	}
	if d[0].operandEnd != 3 {
		t.Errorf("operandEnd = %d, want 3", d[0].operandEnd)
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

func TestParseCFFDictTwoByteIntEncoding(t *testing.T) {
	// Operand 1500 (= 247-250 range produces 108..1131; need 28
	// shortint for 1500). 28 0x05 0xDC = 1500.
	d, err := parseCFFDict([]byte{28, 0x05, 0xDC, 0})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d[0].intOperands[0] != 1500 {
		t.Errorf("operand = %d, want 1500", d[0].intOperands[0])
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
	if len(d[0].realIndices) != 1 || d[0].realIndices[0] != 0 {
		t.Errorf("realIndices = %v", d[0].realIndices)
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
		0x00, 0x0A, // sentinel
	}
	size, err := computeFDSelectSize(raw, 0, 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if size != 11 {
		t.Errorf("size = %d, want 11", size)
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
