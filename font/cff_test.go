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

// fixtureT is the minimal testing-T surface the synthetic CFF builders
// need: Helper for stack-frame attribution and Fatalf to abort on
// invalid options. *testing.T satisfies it; the fuzz seed corpus uses
// a no-op stand-in so it can call the builder outside a test.
type fixtureT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// buildSyntheticCFFCollection assembles a CFF v1 blob with a Top DICT
// INDEX containing two objects. Used to verify that isCIDKeyedCFFv1
// rejects CFF font collections rather than guessing which Top DICT
// applies — sfnt-wrapped OpenType cannot legitimately carry a multi-
// Top-DICT CFF, so the safe answer is "not CID-keyed".
func buildSyntheticCFFCollection(t fixtureT, dict1, dict2 []byte) []byte {
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
	opROS1  byte = 0x0C // first byte of two-byte ROS operator
	opROS2  byte = 30   // second byte: ROS
	opVer   byte = 0    // single-byte 'version' operator
	op4Byte byte = 29   // operand prefix for 4-byte signed int
	op2Byte byte = 28   // operand prefix for 2-byte signed int
	opBCD   byte = 30   // operand prefix for BCD real (same byte as ROS2)
)

// syntheticCFFOptions controls buildSyntheticCIDKeyedCFF. The defaults
// (numGlyphs=3, fdCount=1) produce the smallest valid CID-keyed CFF
// the parser accepts. The builder caps the supported ranges so all
// INDEX sections fit with offSize=1, keeping the resulting bytes
// trivial to hand-verify in test failures.
type syntheticCFFOptions struct {
	numGlyphs int // 1..100
	fdCount   int // 1..20

	// Customisations to drive negative tests. Each "skip" flag omits
	// the named operator from the Top DICT or the per-FD font dict;
	// parseCFF should reject every resulting blob.
	skipROS         bool
	skipCharStrings bool
	skipFDArray     bool
	skipFDSelect    bool
	skipFDPrivate   bool

	// brokenFDPrivate places the FD's Private DICT at an offset past
	// the end of the buffer when true. Exercises the FD private
	// bounds check.
	brokenFDPrivate bool

	// charsetOverride, when non-zero, replaces the Top DICT charset
	// operand with the supplied absolute offset. Used to drive
	// rejection tests for predefined-charset values (0 = ISOAdobe,
	// 1 = Expert, 2 = ExpertSubset) that CID-keyed CFFs must not use
	// per TN #5176 §18.
	charsetOverride    int32
	useCharsetOverride bool

	// charStrings, when non-nil, replaces the default one-endchar-
	// per-glyph charstrings. Length must equal numGlyphs. Used by
	// subset tests to distinguish "kept verbatim" from "replaced
	// with endchar" — the default fixture's identical 0x0E bytes
	// hide that distinction.
	charStrings [][]byte

	// globalSubrs, when non-nil, populates the Global Subr INDEX
	// with the supplied bodies. Used by subset tests to verify
	// reachability-based pruning.
	globalSubrs [][]byte

	// fdSelectFormat3, when non-nil, emits an FDSelect format 3
	// table covering the contiguous ranges supplied. Range
	// boundaries are first-glyph values; the FD applies from
	// that glyph until the next range's first (or numGlyphs).
	// When nil, the builder emits format 0 with FD = gid % fdCount.
	// The first range must start at glyph 0.
	fdSelectFormat3 []syntheticFDRange
}

// syntheticFDRange is one row in an FDSelect format 3 table: the
// glyph index where the range begins and the FD that owns it.
type syntheticFDRange struct {
	firstGID int
	fd       int
}

// buildSyntheticCIDKeyedCFF assembles a complete, valid CID-keyed CFF
// v1 blob deterministically — useful for parser tests that must not
// depend on a system-installed CJK font. All offset operands in the
// Top DICT and per-FD font/private DICTs use the longint (29 + int32)
// encoding so section sizes are independent of the offset values; the
// builder can therefore compute every absolute offset in a single
// forward pass without iterating to a fixed point.
//
// Layout (fixed order, no shared sections):
//
//	Header(4)
//	Name INDEX                ("Test", offSize=1)              -> 9 bytes
//	Top DICT INDEX            (1 entry, fixed 50-byte payload) -> 55 bytes
//	String INDEX              ("Adobe", "Identity")            -> 19 bytes
//	Global Subr INDEX         (empty)                          -> 2 bytes
//	Charset                   (format 0, numGlyphs-1 SIDs)
//	FDSelect                  (format 0, numGlyphs entries)
//	CharStrings INDEX         (each charstring = single byte 0x0E)
//	FDArray INDEX             (per-FD font dict, 11 bytes each)
//	[Private DICT + empty Local Subr INDEX] * fdCount
//
// The Private DICT contains the bare minimum that parseCFF will
// accept: defaultWidthX, nominalWidthX, and Subrs (relative offset
// equal to privateSize, so the empty Local Subr INDEX sits
// immediately after).
func buildSyntheticCIDKeyedCFF(t fixtureT, opts syntheticCFFOptions) []byte {
	t.Helper()
	if opts.numGlyphs < 1 || opts.numGlyphs > 100 {
		t.Fatalf("synthetic: numGlyphs %d out of range [1,100]", opts.numGlyphs)
	}
	if opts.fdCount < 1 || opts.fdCount > 20 {
		t.Fatalf("synthetic: fdCount %d out of range [1,20]", opts.fdCount)
	}

	const (
		headerSize       = 4
		nameIndexSize    = 9
		topDictIndexSize = 55
		stringIndexSize  = 19
		privateDictSize  = 18 // see writePrivateDict
		localSubrSize    = 2  // empty INDEX
		fdEntrySize      = 11 // per-FD font dict
	)

	// Pre-compute sizes that depend on opts.
	// Resolve variable section payloads.
	cs := opts.charStrings
	if cs == nil {
		cs = make([][]byte, opts.numGlyphs)
		for i := range opts.numGlyphs {
			cs[i] = []byte{0x0E}
		}
	}
	if len(cs) != opts.numGlyphs {
		t.Fatalf("synthetic: charStrings length %d != numGlyphs %d", len(cs), opts.numGlyphs)
	}
	charStringsPayload := writeCFFIndex(cs)
	globalSubrPayload := writeCFFIndex(opts.globalSubrs)

	charsetSize := 1 + (opts.numGlyphs-1)*2 // format 0
	fdSelectSize := 1 + opts.numGlyphs      // default: format 0
	if opts.fdSelectFormat3 != nil {
		if len(opts.fdSelectFormat3) == 0 || opts.fdSelectFormat3[0].firstGID != 0 {
			t.Fatalf("synthetic: fdSelectFormat3 must start with a range at glyph 0")
		}
		fdSelectSize = 1 + 2 + len(opts.fdSelectFormat3)*3 + 2
	}
	charStringsSize := len(charStringsPayload)
	gsubrPayloadSize := len(globalSubrPayload)
	fdArraySize := 2 + 1 + (opts.fdCount + 1) + opts.fdCount*fdEntrySize

	// Absolute offsets.
	offNameIndex := headerSize
	offTopDict := offNameIndex + nameIndexSize
	offStringIndex := offTopDict + topDictIndexSize
	offGsubr := offStringIndex + stringIndexSize
	offCharset := offGsubr + gsubrPayloadSize
	offFDSelect := offCharset + charsetSize
	offCharStrings := offFDSelect + fdSelectSize
	offFDArray := offCharStrings + charStringsSize

	// Per-FD blocks come last; record their starting offsets so the
	// FDArray font dicts can point at them and so a broken-private
	// option can target a known-out-of-range address.
	fdPrivateOff := make([]int, opts.fdCount)
	cursor := offFDArray + fdArraySize
	for i := range opts.fdCount {
		fdPrivateOff[i] = cursor
		cursor += privateDictSize + localSubrSize
	}
	totalSize := cursor
	if opts.brokenFDPrivate && opts.fdCount > 0 {
		fdPrivateOff[0] = totalSize + 0x10000 // far beyond the buffer
	}

	// Top DICT body. All operands use longint encoding (29 + int32 BE)
	// to keep the body fixed at exactly topDictPayload bytes.
	const topDictPayload = topDictIndexSize - 5 // 50 bytes
	td := newBytes(t, topDictPayload)
	if !opts.skipROS {
		td.longInt(391) // registry SID (custom string 0)
		td.longInt(392) // ordering SID (custom string 1)
		td.longInt(0)   // supplement
		td.byte(cffOpEscape)
		td.byte(30) // ROS
	}
	td.longInt(int32(opts.numGlyphs))
	td.byte(cffOpEscape)
	td.byte(34) // CIDCount
	charsetOperand := int32(offCharset)
	if opts.useCharsetOverride {
		charsetOperand = opts.charsetOverride
	}
	td.longInt(charsetOperand)
	td.byte(cffOpCharset)
	if !opts.skipCharStrings {
		td.longInt(int32(offCharStrings))
		td.byte(cffOpCharStrings)
	}
	if !opts.skipFDArray {
		td.longInt(int32(offFDArray))
		td.byte(cffOpEscape)
		td.byte(36) // FDArray
	}
	if !opts.skipFDSelect {
		td.longInt(int32(offFDSelect))
		td.byte(cffOpEscape)
		td.byte(37) // FDSelect
	}
	td.padTo(topDictPayload)

	// Assemble the full blob.
	out := make([]byte, 0, totalSize)

	// Header: major=1, minor=0, hdrSize=4, offSize (informational)=1.
	out = append(out, 1, 0, 4, 1)

	// Name INDEX: count=1, offSize=1, offsets=[1,5], data="Test".
	out = append(out, 0x00, 0x01, 0x01, 0x01, 0x05, 'T', 'e', 's', 't')

	// Top DICT INDEX: count=1, offSize=1, offsets=[1, payload+1].
	out = append(out, 0x00, 0x01, 0x01, 0x01, byte(topDictPayload+1))
	out = append(out, td.bytes...)

	// String INDEX: ("Adobe", "Identity") — 5 + 8 = 13 bytes payload.
	out = append(out,
		0x00, 0x02, 0x01,
		0x01, 0x06, 0x0E,
		'A', 'd', 'o', 'b', 'e',
		'I', 'd', 'e', 'n', 't', 'i', 't', 'y',
	)

	// Global Subr INDEX.
	out = append(out, globalSubrPayload...)

	// Charset format 0: SIDs for glyphs 1..numGlyphs-1.
	out = append(out, 0x00)
	for i := 1; i < opts.numGlyphs; i++ {
		out = append(out, byte(i>>8), byte(i&0xFF))
	}

	// FDSelect: format 0 by default, or format 3 when ranges given.
	if opts.fdSelectFormat3 != nil {
		out = append(out, 0x03)
		out = append(out, byte(len(opts.fdSelectFormat3)>>8), byte(len(opts.fdSelectFormat3)&0xFF))
		for _, r := range opts.fdSelectFormat3 {
			out = append(out, byte(r.firstGID>>8), byte(r.firstGID&0xFF), byte(r.fd))
		}
		// Sentinel uint16 = numGlyphs.
		out = append(out, byte(opts.numGlyphs>>8), byte(opts.numGlyphs&0xFF))
	} else {
		out = append(out, 0x00)
		for i := range opts.numGlyphs {
			out = append(out, byte(i%opts.fdCount))
		}
	}

	// CharStrings INDEX.
	out = append(out, charStringsPayload...)

	// FDArray INDEX header.
	out = append(out, 0x00, byte(opts.fdCount), 0x01)
	for i := range opts.fdCount + 1 {
		out = append(out, byte(1+i*fdEntrySize))
	}
	// Per-FD font dicts: longint(privSize) longint(privOff) op:Private.
	for i := range opts.fdCount {
		out = append(out, 29)
		out = appendInt32(out, int32(privateDictSize))
		out = append(out, 29)
		out = appendInt32(out, int32(fdPrivateOff[i]))
		if opts.skipFDPrivate {
			out = append(out, cffOpVersion) // any operator other than Private
		} else {
			out = append(out, cffOpPrivate)
		}
	}

	// Per-FD Private DICTs + empty Local Subr INDEXes.
	for range opts.fdCount {
		// defaultWidthX=0, nominalWidthX=0, Subrs at relative
		// offset = privateDictSize.
		out = append(out, 29)
		out = appendInt32(out, 0)
		out = append(out, cffOpDefaultWidthX)
		out = append(out, 29)
		out = appendInt32(out, 0)
		out = append(out, cffOpNominalWidthX)
		out = append(out, 29)
		out = appendInt32(out, int32(privateDictSize))
		out = append(out, cffOpSubrs)
		// Local Subr INDEX (empty).
		out = append(out, 0x00, 0x00)
	}

	if len(out) != totalSize {
		t.Fatalf("synthetic: produced %d bytes, expected %d", len(out), totalSize)
	}
	return out
}

// appendInt32 appends a 4-byte big-endian int32. Inlined here to keep
// the synthetic builder readable.
func appendInt32(b []byte, v int32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// bytesBuilder is a fixed-capacity byte buffer used by the synthetic
// CFF builder. It exists to make the layered Top DICT construction
// readable: each operand/operator emit is a single named call, and a
// final padTo zero-fills to the declared payload size.
type bytesBuilder struct {
	t     fixtureT
	bytes []byte
	cap   int
}

func newBytes(t fixtureT, cap int) *bytesBuilder {
	return &bytesBuilder{t: t, cap: cap}
}

func (b *bytesBuilder) byte(v byte) {
	if len(b.bytes)+1 > b.cap {
		b.t.Fatalf("bytesBuilder: overflow at byte(%d)", v)
	}
	b.bytes = append(b.bytes, v)
}

func (b *bytesBuilder) longInt(v int32) {
	if len(b.bytes)+5 > b.cap {
		b.t.Fatalf("bytesBuilder: overflow at longInt(%d)", v)
	}
	b.bytes = append(b.bytes, 29)
	b.bytes = appendInt32(b.bytes, v)
}

func (b *bytesBuilder) padTo(n int) {
	for len(b.bytes) < n {
		b.bytes = append(b.bytes, 0)
	}
}

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
