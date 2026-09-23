// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"runtime/metrics"
	"strings"
	"testing"
	"time"
	"unsafe"
)

// Every input in this file is small. Each test asserts a bound on the
// bytes that the call under test allocates (runtime.MemStats.TotalAlloc).
// The total allocation is an upper bound on the peak heap of the call, and
// it does not depend on when the GC runs. Where a regression would make an
// input grow without bound (form XObject fan-out), the test runs in a child
// process that a heap watchdog stops at 256 MiB.

// allocatedBy returns the bytes that f allocates.
func allocatedBy(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// assertAllocBelow fails the test if f allocates limit bytes or more.
func assertAllocBelow(t *testing.T, limit uint64, f func()) {
	t.Helper()
	n := allocatedBy(f)
	if n >= limit {
		t.Errorf("allocated %.2f MiB, want less than %.2f MiB", float64(n)/(1<<20), float64(limit)/(1<<20))
		return
	}
	t.Logf("allocated %.2f MiB (bound %.2f MiB)", float64(n)/(1<<20), float64(limit)/(1<<20))
}

func deflate(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

// testObj is one indirect object for testPDF: a dictionary, or a stream
// when data is not nil. A stream's dict holds the entries between << >>.
type testObj struct {
	dict string
	data []byte
}

// testPDF writes objs as objects 1, 2, ... with a classic xref table.
// Object 1 must be the catalog.
func testPDF(objs ...testObj) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n", i+1)
		if o.data != nil {
			fmt.Fprintf(&b, "<< %s /Length %d >>\nstream\n", o.dict, len(o.data))
			b.Write(o.data)
			b.WriteString("\nendstream\nendobj\n")
		} else {
			fmt.Fprintf(&b, "%s\nendobj\n", o.dict)
		}
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// onePagePDF is a one-page PDF whose page has the given /Contents and
// /Resources values. Object 4 is a Flate stream of content; extra
// objects follow as 5, 6, ...
func onePagePDF(t *testing.T, contents, resources string, content []byte, extra ...testObj) []byte {
	t.Helper()
	objs := []testObj{
		{dict: "<< /Type /Catalog /Pages 2 0 R >>"},
		{dict: "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		{dict: "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents " + contents + " /Resources " + resources + " >>"},
		{dict: "/Filter /FlateDecode", data: deflate(t, content)},
	}
	return testPDF(append(objs, extra...)...)
}

func firstPage(t *testing.T, data []byte, limits MemoryLimits) *PageInfo {
	t.Helper()
	r, err := ParseWithOptions(data, ReadOptions{MemoryLimits: limits})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p, err := r.Page(0)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	return p
}

func wantLimitErr(t *testing.T, err error, substr string) {
	t.Helper()
	if !errors.Is(err, ErrMemoryLimitExceeded) {
		t.Fatalf("error = %v, want ErrMemoryLimitExceeded", err)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("error = %q, want it to contain %q", err, substr)
	}
}

// --- The Token is 48 bytes -------------------------------------------------

func TestTokenIs48Bytes(t *testing.T) {
	if got := unsafe.Sizeof(Token{}); got != tokenSize {
		t.Errorf("unsafe.Sizeof(Token{}) = %d, want %d", got, tokenSize)
	}
}

// --- Vector 1: ParseContentStream kept every operand -------------------------

// TestParseContentStreamOperandLimit: a run of operands with no operator
// stops the parse at 1025 operands. "1 " repeated 1M times (2 MB) used to
// keep 1M 56-byte tokens (about 140 MB allocated).
func TestParseContentStreamOperandLimit(t *testing.T) {
	data := bytes.Repeat([]byte("1 "), 1<<20)
	var ops []ContentOp
	var err error
	assertAllocBelow(t, 1<<20, func() { ops, err = ParseContentStreamWithLimits(data, MemoryLimits{}) })
	wantLimitErr(t, err, "more than 1024 operands")
	if len(ops) != 0 {
		t.Errorf("got %d ops, want 0", len(ops))
	}

	// The limit applies to ParseContentStream too, which drops the error.
	assertAllocBelow(t, 1<<20, func() { ops = ParseContentStream(data) })
	if len(ops) != 0 {
		t.Errorf("ParseContentStream: got %d ops, want 0", len(ops))
	}
}

// TestParseContentStreamOperandLimitEdge: 1024 operands are kept; an
// array or a dictionary counts as one operand.
func TestParseContentStreamOperandLimitEdge(t *testing.T) {
	at := strings.Repeat("1 ", 1023) + "[1 2 3] n"
	ops, err := ParseContentStreamWithLimits([]byte(at), MemoryLimits{})
	if err != nil || len(ops) != 1 || len(ops[0].Operands) != 1023+5 {
		t.Fatalf("1024 operands: %d ops, err %v", len(ops), err)
	}
	over := strings.Repeat("1 ", 1024) + "<< /A 1 >> n"
	_, err = ParseContentStreamWithLimits([]byte(over), MemoryLimits{})
	wantLimitErr(t, err, "more than 1024 operands")
}

// TestParseContentStreamOperandTokenLimit: one operator may have 65536
// operand tokens, array contents included, so a long TJ array still parses.
func TestParseContentStreamOperandTokenLimit(t *testing.T) {
	long := "[" + strings.Repeat("(a) -20 ", 32767) + "] TJ" // 65536 tokens
	ops, err := ParseContentStreamWithLimits([]byte(long), MemoryLimits{})
	if err != nil || len(ops) != 1 || len(ops[0].Operands) != 65536 {
		t.Fatalf("65536-token TJ: %d ops, err %v", len(ops), err)
	}
	longer := "[" + strings.Repeat("(a) -20 ", 32768) + "] TJ"
	_, err = ParseContentStreamWithLimits([]byte(longer), MemoryLimits{})
	wantLimitErr(t, err, "more than 65536 operand tokens")
}

// TestParseContentStreamTokenBudget: MaxContentTokens bounds the operators
// plus operands that a parse keeps. "q Q " repeated 4M times used to keep
// 8M ops (926 MiB peak); here the same shape stops at a 10,000 token limit.
func TestParseContentStreamTokenBudget(t *testing.T) {
	data := bytes.Repeat([]byte("q Q "), 20_000)
	limits := MemoryLimits{MaxContentTokens: 10_000}
	var ops []ContentOp
	var err error
	assertAllocBelow(t, 4<<20, func() { ops, err = ParseContentStreamWithLimits(data, limits) })
	wantLimitErr(t, err, "content streams hold more than 10000 tokens")
	if len(ops) != 10_000 {
		t.Errorf("got %d ops, want the 10000 before the limit", len(ops))
	}

	// Exactly at the limit: no error.
	ops, err = ParseContentStreamWithLimits(bytes.Repeat([]byte("1 0 0 1 0 0 cm "), 1000), MemoryLimits{MaxContentTokens: 7000})
	if err != nil || len(ops) != 1000 {
		t.Errorf("at the limit: %d ops, err %v", len(ops), err)
	}
	// -1 disables the limit.
	if _, err := ParseContentStreamWithLimits(data, MemoryLimits{MaxContentTokens: -1}); err != nil {
		t.Errorf("disabled limit: %v", err)
	}
	if got := (MemoryLimits{}).effectiveMaxContentTokens(); got != defaultMaxContentTokens {
		t.Errorf("default MaxContentTokens = %d, want %d", got, defaultMaxContentTokens)
	}
}

// TestParseContentStreamOperandsAreIndependent: ops share one operand
// arena, so an append to one op's operands must not change the next op.
func TestParseContentStreamOperandsAreIndependent(t *testing.T) {
	ops := ParseContentStream([]byte("1 2 m 3 4 l"))
	if len(ops) != 2 {
		t.Fatalf("got %d ops", len(ops))
	}
	_ = append(ops[0].Operands, Token{Type: TokenNumber, Value: "9"})
	if ops[1].Operands[0].Value != "3" {
		t.Errorf("append to op 0 changed op 1: %+v", ops[1].Operands)
	}
}

// TestPageExtractTextOperandFlood: the vector through the caller. A page
// of "1 " repeated 1M times (a 2 KB file) returns no text and an error,
// not hundreds of megabytes of tokens.
func TestPageExtractTextOperandFlood(t *testing.T) {
	p := firstPage(t, onePagePDF(t, "4 0 R", "<< >>", bytes.Repeat([]byte("1 "), 1<<20)), MemoryLimits{})
	var text string
	var err error
	// The decode of the 2 MB stream allocates about 6 MB; the parse must add little.
	assertAllocBelow(t, 12<<20, func() { text, err = p.ExtractText() })
	wantLimitErr(t, err, "more than 1024 operands")
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
}

// TestPageExtractTextPartial: past a limit, ExtractText returns the text
// before it with the error, so a caller can keep what was found.
func TestPageExtractTextPartial(t *testing.T) {
	var content strings.Builder
	content.WriteString("BT /F1 12 Tf ")
	for i := range 1000 {
		fmt.Fprintf(&content, "0 -14 Td (line%d) Tj ", i)
	}
	content.WriteString("ET")
	data := onePagePDF(t, "4 0 R", "<< >>", []byte(content.String()))

	full, err := firstPage(t, data, MemoryLimits{}).ExtractText()
	if err != nil {
		t.Fatalf("no limit: %v", err)
	}
	part, err := firstPage(t, data, MemoryLimits{MaxContentTokens: 2000}).ExtractText()
	wantLimitErr(t, err, "content streams hold more than 2000 tokens")
	if part == "" || len(part) >= len(full) || !strings.HasPrefix(full, part) {
		t.Errorf("partial text (%d bytes) is not a proper prefix of the full text (%d bytes)", len(part), len(full))
	}

	ops, err := firstPage(t, data, MemoryLimits{MaxContentTokens: 2000}).ContentOps()
	wantLimitErr(t, err, "content streams hold")
	if len(ops) == 0 {
		t.Error("ContentOps returned no ops before the limit")
	}
}

// --- Vector 2: ContentStream multiplied repeated /Contents references -------

// TestContentStreamRepeatedReferenceLimit: a 64 KiB stream listed 200 times
// is a 13 MB concatenation. With MaxStreamSize at 1 MiB, ContentStream
// refuses it before it allocates the result.
func TestContentStreamRepeatedReferenceLimit(t *testing.T) {
	refs := "[" + strings.Repeat("4 0 R ", 200) + "]"
	data := onePagePDF(t, refs, "<< >>", bytes.Repeat([]byte("q Q "), 16<<10))
	p := firstPage(t, data, MemoryLimits{MaxStreamSize: 1 << 20})
	var cs []byte
	var err error
	assertAllocBelow(t, 1<<20, func() { cs, err = p.ContentStream() })
	wantLimitErr(t, err, "page content streams total 13107400 bytes, limit 1048576")
	if cs != nil {
		t.Errorf("got %d bytes, want nil", len(cs))
	}
}

// TestContentStreamRepeatedReferenceCharged: a stream listed again is
// decoded and charged once, but each extra copy is new memory, so the
// copies are charged to MaxTotalAlloc.
func TestContentStreamRepeatedReferenceCharged(t *testing.T) {
	refs := "[" + strings.Repeat("4 0 R ", 100) + "]" // 6.4 MB, under MaxStreamSize
	data := onePagePDF(t, refs, "<< >>", bytes.Repeat([]byte("q Q "), 16<<10))
	p := firstPage(t, data, MemoryLimits{MaxStreamSize: 16 << 20, MaxTotalAlloc: 4 << 20})
	_, err := p.ContentStream()
	wantLimitErr(t, err, "total allocation")
}

// TestContentStreamConcatenation: the concatenation is unchanged: each
// stream in order, each followed by a newline, repeats included.
func TestContentStreamConcatenation(t *testing.T) {
	data := testPDF(
		testObj{dict: "<< /Type /Catalog /Pages 2 0 R >>"},
		testObj{dict: "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		testObj{dict: "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Contents [4 0 R 5 0 R 4 0 R 9 0 R] >>"},
		testObj{dict: "", data: []byte("q")},
		testObj{dict: "", data: []byte("1 0 0 1 5 5 cm")},
	)
	cs, err := firstPage(t, data, MemoryLimits{}).ContentStream()
	if err != nil {
		t.Fatal(err)
	}
	if want := "q\n1 0 0 1 5 5 cm\nq\n"; string(cs) != want {
		t.Errorf("ContentStream = %q, want %q", cs, want)
	}
}

// --- Siblings: the content processor --------------------------------------

// TestContentProcessorStateNesting: q saves at most maxStateNesting states,
// and a Q for a q that saved nothing restores nothing, so the stack stays
// balanced. 20,000 q used to keep 20,000 264-byte states.
func TestContentProcessorStateNesting(t *testing.T) {
	const n = 20_000
	content := strings.Repeat("q ", n) + "2 0 0 2 50 50 cm " + strings.Repeat("Q ", n) + "BT (x) Tj ET"
	ops := ParseContentStream([]byte(content))
	proc := NewContentProcessor(nil)
	var spans []TextSpan
	assertAllocBelow(t, 4<<20, func() { spans = proc.Process(ops) })
	if len(proc.stack) != 0 || proc.lostSaves != 0 {
		t.Errorf("stack %d, lost saves %d after balanced q/Q, want 0 and 0", len(proc.stack), proc.lostSaves)
	}
	if len(spans) != 1 || spans[0].Matrix != [6]float64{1, 0, 0, 1, 0, 0} {
		t.Errorf("span matrix after balanced q/Q = %v, want identity", spans)
	}

	proc.Process(ParseContentStream([]byte(strings.Repeat("q ", n))))
	if len(proc.stack) != maxStateNesting {
		t.Errorf("stack depth = %d, want %d", len(proc.stack), maxStateNesting)
	}
}

// TestContentProcessorResultsBudget: results count against MaxContentTokens
// by size. A stream of "h" (1 token each) makes a path segment for each.
func TestContentProcessorResultsBudget(t *testing.T) {
	content := "0 0 m " + strings.Repeat("h ", 20_000) + "S"
	ops := ParseContentStream([]byte(content))
	proc := NewContentProcessor(nil)
	proc.SetMemoryLimits(MemoryLimits{MaxContentTokens: 30_000})
	assertAllocBelow(t, 4<<20, func() { proc.Process(ops) })
	wantLimitErr(t, proc.Err(), "content results take more than 30000 tokens")

	// The same content fits a larger budget and gives every segment.
	proc.SetMemoryLimits(MemoryLimits{MaxContentTokens: 70_000})
	proc.Process(ops)
	if proc.Err() != nil || len(proc.Paths()) != 20_001 {
		t.Errorf("larger budget: %d paths, err %v", len(proc.Paths()), proc.Err())
	}
}

// TestContentProcessorWalkBudget: the tokens that Process walks are bounded,
// and Err is reset by the next Process call.
func TestContentProcessorWalkBudget(t *testing.T) {
	ops := ParseContentStream(bytes.Repeat([]byte("1 0 0 1 0 0 cm "), 1000)) // 7000 tokens
	proc := NewContentProcessor(nil)
	proc.SetMemoryLimits(MemoryLimits{MaxContentTokens: 3500})
	proc.Process(ops)
	wantLimitErr(t, proc.Err(), "content walk passes more than 3500 tokens")
	proc.Process(ops[:10])
	if proc.Err() != nil {
		t.Errorf("Err after a Process within the limit = %v, want nil", proc.Err())
	}
}

// formFanoutPDF has a form XObject that draws itself twice. The walk
// recursed 50 levels deep with 2^50 draws and ran past 3 GiB.
func formFanoutPDF(t *testing.T) []byte {
	form := testObj{dict: "/Type /XObject /Subtype /Form /BBox [0 0 1 1] /Filter /FlateDecode", data: deflate(t, []byte("/F Do /F Do"))}
	return onePagePDF(t, "4 0 R", "<< /XObject << /F 5 0 R >> >>", []byte("/F Do"), form)
}

// TestFormXObjectFanout runs in a child process: if the walk budget were
// lost, the fan-out would grow without bound, and the watchdog in the child
// stops it at 256 MiB instead of the whole test run.
func TestFormXObjectFanout(t *testing.T) {
	if os.Getenv("FOLIO_FANOUT_CHILD") == "1" {
		stopAboveHeap(256 << 20)
		p := firstPage(t, formFanoutPDF(t), MemoryLimits{MaxContentTokens: 20_000})
		refs, err := p.ImageRefs()
		wantLimitErr(t, err, "more than 20000 tokens")
		if len(refs) == 0 || len(refs) > 10_000 {
			t.Errorf("got %d image refs, want 1 to 10000", len(refs))
		}
		spans, err := p.TextSpans()
		wantLimitErr(t, err, "more than 20000 tokens")
		_ = spans
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestFormXObjectFanout$", "-test.v")
	cmd.Env = append(os.Environ(), "FOLIO_FANOUT_CHILD=1")
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		<-done
		t.Fatalf("child did not finish within 60s:\n%s", out)
	}
	if err != nil {
		t.Fatalf("child failed: %v\n%s", err, out)
	}
}

// stopAboveHeap exits the process with status 3 if the live heap passes limit.
func stopAboveHeap(limit uint64) {
	go func() {
		s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
		for {
			metrics.Read(s)
			if s[0].Value.Uint64() > limit {
				fmt.Fprintf(os.Stderr, "heap passed %d bytes; stopping\n", limit)
				os.Exit(3)
			}
			time.Sleep(time.Millisecond)
		}
	}()
}

// TestFormXObjectsShareParseBudget: the page and all of its forms are
// parsed against one MaxContentTokens budget.
func TestFormXObjectsShareParseBudget(t *testing.T) {
	var names, xobjects strings.Builder
	var forms []testObj
	formContent := deflate(t, bytes.Repeat([]byte("1 0 0 1 0 0 cm "), 100)) // 700 tokens
	for i := range 10 {
		fmt.Fprintf(&names, "/F%d Do ", i)
		fmt.Fprintf(&xobjects, "/F%d %d 0 R ", i, 5+i)
		forms = append(forms, testObj{dict: "/Type /XObject /Subtype /Form /BBox [0 0 1 1] /Filter /FlateDecode", data: formContent})
	}
	data := onePagePDF(t, "4 0 R", "<< /XObject << "+xobjects.String()+">> >>", []byte(names.String()), forms...)

	// The walk needs 20 + 10*700 = 7020 tokens and the parses as many.
	if _, err := firstPage(t, data, MemoryLimits{MaxContentTokens: 8000}).PathOps(); err != nil {
		t.Fatalf("within the limit: %v", err)
	}
	// The parse budget runs out at form 8 of 10.
	_, err := firstPage(t, data, MemoryLimits{MaxContentTokens: 5000}).PathOps()
	wantLimitErr(t, err, "content streams hold more than 5000 tokens")
}

// --- Siblings: decoded text -------------------------------------------------

// TestToUnicodeDestinationLimit: a ToUnicode destination is cut at 256
// UTF-16 code units.
func TestToUnicodeDestinationLimit(t *testing.T) {
	cm := ParseCMap([]byte("1 begincodespacerange <00> <FF> endcodespacerange\n" +
		"1 beginbfchar\n<01> <" + strings.Repeat("0041", 5000) + ">\nendbfchar\n"))
	if got := cm.Decode([]byte{1}); got != strings.Repeat("A", maxToUnicodeDstUnits) {
		t.Errorf("decoded %d bytes, want %d", len(got), maxToUnicodeDstUnits)
	}
}

// toUnicodePDF has font F1 whose ToUnicode maps code 01 to 256 'A's, and
// a page that shows n such codes.
func toUnicodePDF(t *testing.T, n int) []byte {
	cmap := "1 begincodespacerange <00> <FF> endcodespacerange\n1 beginbfchar\n<01> <" +
		strings.Repeat("0041", 256) + ">\nendbfchar\n"
	font := testObj{dict: "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /ToUnicode 6 0 R >>"}
	toUnicode := testObj{dict: "/Filter /FlateDecode", data: deflate(t, []byte(cmap))}
	content := "BT /F1 12 Tf <" + strings.Repeat("01", n) + "> Tj ET"
	return onePagePDF(t, "4 0 R", "<< /Font << /F1 5 0 R >> >>", []byte(content), font, toUnicode)
}

// TestExtractTextDecodeBudget: decoded text counts against MaxContentTokens
// at one token per 48 bytes, and a decode is cut before it runs, so it never
// makes more text than the budget allows. 4,000 codes of 256 characters
// would be 1 MB of text; the limit of 1,000 tokens allows 48,000 bytes.
func TestExtractTextDecodeBudget(t *testing.T) {
	data := toUnicodePDF(t, 4000)
	p := firstPage(t, data, MemoryLimits{MaxContentTokens: 1000})
	var text string
	var err error
	assertAllocBelow(t, 2<<20, func() { text, err = p.ExtractText() })
	wantLimitErr(t, err, "extracted text takes more than 1000 tokens")
	if len(text) == 0 || len(text) > 1000*tokenSize {
		t.Errorf("text is %d bytes, want 1 to %d", len(text), 1000*tokenSize)
	}

	spans, err := firstPage(t, data, MemoryLimits{MaxContentTokens: 1000}).TextSpans()
	wantLimitErr(t, err, "content results take more than 1000 tokens")
	if len(spans) != 1 || len(spans[0].Text) > 1000*tokenSize {
		t.Errorf("spans = %d, want 1 span of at most %d bytes", len(spans), 1000*tokenSize)
	}

	// Under the default limits the whole text comes out.
	text, err = firstPage(t, data, MemoryLimits{}).ExtractText()
	if err != nil || len(text) != 4000*256 {
		t.Errorf("default limits: %d bytes, err %v", len(text), err)
	}
}

// --- Siblings: redaction must not act on a partial parse --------------------

func TestRedactionRefusesTruncatedContent(t *testing.T) {
	data := bytes.Repeat([]byte("1 0 0 1 0 0 cm "), 100)
	out, err := rewriteContentStream(data, []Box{{0, 0, 10, 10}}, nil, MemoryLimits{MaxContentTokens: 50})
	wantLimitErr(t, err, "content streams hold")
	if out != nil {
		t.Errorf("rewrite returned %d bytes, want nil", len(out))
	}

	pdf := onePagePDF(t, "4 0 R", "<< >>", []byte("BT /F1 12 Tf 10 10 Td (secret) Tj ET "+strings.Repeat("0 0 m ", 100)))
	r, err := ParseWithOptions(pdf, ReadOptions{MemoryLimits: MemoryLimits{MaxContentTokens: 50}})
	if err != nil {
		t.Fatal(err)
	}
	// The glyph search fails on its own, before any rewrite.
	glyphs, err := pageGlyphs(r, 0)
	wantLimitErr(t, err, "content streams hold")
	if glyphs != nil {
		t.Errorf("pageGlyphs returned %d glyphs, want nil", len(glyphs))
	}
	if _, err := RedactText(r, []string{"secret"}, nil); !errors.Is(err, ErrMemoryLimitExceeded) {
		t.Errorf("RedactText error = %v, want ErrMemoryLimitExceeded", err)
	}
}

// --- Normal content parses as it did ----------------------------------------

// legacyParseContentStream is ParseContentStream as it was before the
// limits (v0.9.1-fulcrum.33), kept to check that normal content parses
// the same.
func legacyParseContentStream(data []byte) []ContentOp {
	tok := NewTokenizer(data)
	var ops []ContentOp
	var operands []Token
	for {
		token := tok.Next()
		if token.Type == TokenEOF {
			break
		}
		switch token.Type {
		case TokenKeyword:
			if token.Value == "BI" {
				skipInlineImage(tok)
				operands = nil
				continue
			}
			ops = append(ops, ContentOp{Operator: token.Value, Operands: operands})
			operands = nil
		default:
			operands = append(operands, token)
		}
	}
	return ops
}

func TestParseContentStreamMatchesLegacy(t *testing.T) {
	samples := []string{
		"",
		"BT /F1 12 Tf 100 700 Td (Hello World) Tj ET",
		"q 1 0 0 1 72 72 cm 0 0 m 100 0 l 100 100 l h S Q",
		"BT [(A) -120 (W) 30.5 <0041> -250 (x)] TJ ET",
		"/P << /MCID 3 /Alt (a [b] c) >> BDC BT (t) Tj ET EMC",
		"[3 2] 0 d 0.5 w 1 J 0 j 10 M",
		"q BI /W 2 /H 2 /CS /G /BPC 8 ID \x00\x01\x02\x03 EI Q 1 g",
		"(esc \\( \\) \\\\ \\n \\101) Tj % comment\n(next) '",
		"1 2 3 4 5 6 cm ) { } 7 8 m",
		"10 20 re f* 1 0 0 RG 0.1 0.2 0.3 0.4 k /Pattern cs /P1 scn",
		"[[1 2] [3 [4 5]]] 6 dangling-op",
		"1 2 3", // operands with no operator are dropped
		"BT 0 Tr 12 TL T* (a) ' 1 2 (b) \" ET",
	}
	// Content streams of the PDFs that other reader tests generate.
	for _, pdf := range [][]byte{generateTestPDF(t), generateLayoutPDF(t), createTestPDF(t, "alpha", "beta"), makePDF(t, "t", 3)} {
		r, err := Parse(pdf)
		if err != nil {
			t.Fatal(err)
		}
		for i := range r.PageCount() {
			p, _ := r.Page(i)
			cs, err := p.ContentStream()
			if err != nil {
				t.Fatal(err)
			}
			samples = append(samples, string(cs))
		}
	}
	for i, s := range samples {
		want := legacyParseContentStream([]byte(s))
		got, err := ParseContentStreamWithLimits([]byte(s), MemoryLimits{})
		if err != nil {
			t.Errorf("sample %d: error %v", i, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("sample %d (%.60q): ops differ\n got  %+v\n want %+v", i, s, got, want)
		}
	}
}
