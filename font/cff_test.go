// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "testing"

// buildSyntheticCFFv1 assembles a minimal CFF v1 blob whose first Top
// DICT body is `topDict`. The Header, Name INDEX, and Top DICT INDEX
// scaffolding are filled in with the smallest valid values: a single
// "Test" name entry and a single Top DICT object. The function returns
// raw CFF bytes — not an OpenType wrapper — matching what `CFFData()`
// would yield for a real face.
//
// This fixture exists to give isCIDKeyedCFFv1 deterministic coverage
// of the Top DICT operand/operator stream without depending on a
// system-provided CJK font. Adobe TN #5176 §5 (Header), §5 INDEX
// format, and §9 Top DICT operators are the relevant references.
func buildSyntheticCFFv1(topDict []byte) []byte {
	out := []byte{
		// Header: major=1, minor=0, hdrSize=4, offSize=2.
		// offSize here is the absolute-offset width for any
		// references emitted into the Top DICT itself; isCIDKeyedCFFv1
		// does not chase those, so the value is informational.
		1, 0, 4, 2,
		// Name INDEX: count=1, offSize=1, offsets=[1,5], data="Test".
		0x00, 0x01,
		0x01,
		0x01, 0x05,
		'T', 'e', 's', 't',
		// Top DICT INDEX header: count=1, offSize=1,
		// offsets=[1, len(topDict)+1].
		0x00, 0x01,
		0x01,
		0x01, byte(len(topDict) + 1),
	}
	out = append(out, topDict...)
	return out
}

// buildSyntheticCFFCollection assembles a CFF v1 blob with a Top DICT
// INDEX containing two objects. Used to verify that isCIDKeyedCFFv1
// rejects CFF font collections rather than guessing which Top DICT
// applies — sfnt-wrapped OpenType cannot legitimately carry a multi-
// Top-DICT CFF, so the safe answer is "not CID-keyed".
func buildSyntheticCFFCollection(t *testing.T, dict1, dict2 []byte) []byte {
	t.Helper()
	if len(dict1)+1 > 0xFF || len(dict1)+1+len(dict2) > 0xFF {
		t.Fatalf("synthetic dicts exceed 1-byte offset range")
	}
	out := []byte{
		1, 0, 4, 2,
		0x00, 0x01, 0x01, 0x01, 0x05, 'T', 'e', 's', 't',
		// Top DICT INDEX with count=2.
		0x00, 0x02,
		0x01,
		0x01, byte(len(dict1) + 1), byte(len(dict1) + 1 + len(dict2)),
	}
	out = append(out, dict1...)
	out = append(out, dict2...)
	return out
}

// Operand encodings used in the test dicts. Per TN #5176 §4 the byte
// 139 encodes integer 0 (single-byte form, value = b0 - 139).
const (
	opInt0  byte = 139  // integer 0
	opROS1       = 0x0C // first byte of two-byte ROS operator
	opROS2       = 30   // second byte: ROS
	opVer        = 0    // single-byte 'version' operator
	op4Byte      = 29   // operand prefix for 4-byte signed int
	op2Byte      = 28   // operand prefix for 2-byte signed int
	opBCD        = 30   // operand prefix for BCD real (same byte as ROS2)
)

func TestIsCIDKeyedCFFv1Positive(t *testing.T) {
	// Three integer operands then ROS — the canonical CID-keyed Top
	// DICT opener.
	dict := []byte{opInt0, opInt0, opInt0, opROS1, opROS2}
	cff := buildSyntheticCFFv1(dict)
	if !isCIDKeyedCFFv1(cff) {
		t.Fatal("expected CID-keyed CFF to be accepted")
	}
}

func TestIsCIDKeyedCFFv1NameKeyedFirstOperator(t *testing.T) {
	// 'version' is the first operator — name-keyed CFF, not CID.
	dict := []byte{opInt0, opVer}
	cff := buildSyntheticCFFv1(dict)
	if isCIDKeyedCFFv1(cff) {
		t.Error("expected name-keyed CFF to be rejected")
	}
}

func TestIsCIDKeyedCFFv1ROSBytesInsideOperand(t *testing.T) {
	// 4-byte integer operand whose payload contains the bytes
	// `0x0C 0x1E` followed by a 'version' operator. A naive byte
	// scan would find ROS and falsely return true; the operand-aware
	// parser must skip the int and read 'version' as the first
	// operator.
	dict := []byte{op4Byte, 0x00, opROS1, opROS2, 0x00, opVer}
	cff := buildSyntheticCFFv1(dict)
	if isCIDKeyedCFFv1(cff) {
		t.Error("operand payload mimicking ROS must not be misread")
	}
}

func TestIsCIDKeyedCFFv1TwoByteIntCarryingROSBytes(t *testing.T) {
	// 2-byte integer operand whose payload happens to be `0x0C 0x1E`,
	// followed by 'version'.
	dict := []byte{op2Byte, opROS1, opROS2, opVer}
	cff := buildSyntheticCFFv1(dict)
	if isCIDKeyedCFFv1(cff) {
		t.Error("2-byte int payload mimicking ROS must not be misread")
	}
}

func TestIsCIDKeyedCFFv1BCDOperand(t *testing.T) {
	// BCD real operand encodes the value "1E" (decimal) and then
	// ends with the nibble 0xF. The parser must walk past the BCD
	// bytes rather than treat them as int payload or operator.
	// Bytes: 30 (BCD prefix), 0x1E (digits "1E" — nibble 1 then E
	// which is 'minus' marker), 0xFF (end marker filling both
	// nibbles). Then a 'version' operator.
	dict := []byte{opBCD, 0x1E, 0xFF, opVer}
	cff := buildSyntheticCFFv1(dict)
	if isCIDKeyedCFFv1(cff) {
		t.Error("BCD operand must not be misparsed as ROS")
	}
}

func TestIsCIDKeyedCFFv1CFF2Major(t *testing.T) {
	cff := buildSyntheticCFFv1([]byte{opInt0, opInt0, opInt0, opROS1, opROS2})
	cff[0] = 2 // claim CFF2 major version
	if isCIDKeyedCFFv1(cff) {
		t.Error("CFF major != 1 must be rejected")
	}
}

func TestIsCIDKeyedCFFv1TruncatedHeader(t *testing.T) {
	for n := range 4 {
		buf := make([]byte, n)
		if isCIDKeyedCFFv1(buf) {
			t.Errorf("len=%d should not detect CID-keyed", n)
		}
	}
}

func TestIsCIDKeyedCFFv1HeaderBadHdrSize(t *testing.T) {
	cff := buildSyntheticCFFv1([]byte{opInt0, opInt0, opInt0, opROS1, opROS2})
	cff[2] = 3 // hdrSize < 4 is invalid
	if isCIDKeyedCFFv1(cff) {
		t.Error("hdrSize < 4 must be rejected")
	}
	cff[2] = 200 // hdrSize past EOF
	if isCIDKeyedCFFv1(cff) {
		t.Error("hdrSize past EOF must be rejected")
	}
}

func TestIsCIDKeyedCFFv1TruncatedTopDICTINDEX(t *testing.T) {
	// Build a valid header + Name INDEX, then chop off everything
	// after.
	full := buildSyntheticCFFv1([]byte{opInt0, opInt0, opInt0, opROS1, opROS2})
	for cut := len(full) - 1; cut >= 4+9; cut-- {
		if isCIDKeyedCFFv1(full[:cut]) {
			t.Errorf("cut=%d must not detect CID-keyed", cut)
		}
	}
}

func TestIsCIDKeyedCFFv1RejectsCollection(t *testing.T) {
	dict := []byte{opInt0, opInt0, opInt0, opROS1, opROS2}
	cff := buildSyntheticCFFCollection(t, dict, dict)
	if isCIDKeyedCFFv1(cff) {
		t.Error("Top DICT INDEX count > 1 must be rejected")
	}
}

func TestIsCIDKeyedCFFv1EmptyTopDICT(t *testing.T) {
	// Top DICT body of zero bytes. The INDEX is still valid: offsets
	// = [1, 1]. The walker exits the loop immediately with no
	// operator found.
	cff := buildSyntheticCFFv1(nil)
	if isCIDKeyedCFFv1(cff) {
		t.Error("empty Top DICT must be rejected")
	}
}

func TestIsCIDKeyedCFFv1ReservedByte(t *testing.T) {
	// Byte 22 is reserved (per TN #5176 §4: bytes 22..27 and 31 are
	// reserved/unused in the operand/operator encoding). Hitting one
	// is a parse error, not a CID indicator.
	dict := []byte{22, opROS1, opROS2}
	cff := buildSyntheticCFFv1(dict)
	if isCIDKeyedCFFv1(cff) {
		t.Error("reserved byte in Top DICT must abort detection")
	}
}
