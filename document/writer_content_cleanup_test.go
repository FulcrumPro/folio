// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// addPageWithContents builds a Page dict whose /Contents points at
// the given content stream, and wires the Page into a kids array
// reachable from /Root via the Pages tree.
func addPageWithContents(t *testing.T, w *Writer, contentBytes []byte) (
	pageRef, contentRef *core.PdfIndirectReference,
	contentStream *core.PdfStream,
) {
	t.Helper()
	contentStream = core.NewPdfStream(contentBytes)
	contentRef = w.AddObject(contentStream)

	page := core.NewPdfDictionary()
	page.Set("Type", core.NewPdfName("Page"))
	page.Set("Contents", contentRef)
	pageRef = w.AddObject(page)

	pagesDict := w.objects[1].Object.(*core.PdfDictionary) // catalog=1, pages=2
	kids := pagesDict.Get("Kids").(*core.PdfArray)
	kids.Add(pageRef)
	pagesDict.Set("Count", core.NewPdfInteger(kids.Len()))
	page.Set("Parent", w.objects[1].Object.(*core.PdfDictionary).Get("Type")) // dummy parent ok for test

	return pageRef, contentRef, contentStream
}

// --- Direct lexer tests ---

func TestScanContentTokens_BasicOperators(t *testing.T) {
	data := []byte("q\n1 0 0 1 50 50 cm\nQ\n")
	tokens := scanContentTokens(data)
	if len(tokens) != 9 {
		t.Fatalf("got %d tokens, want 9 (q + 6 numbers + cm + Q)", len(tokens))
	}
	if string(tokens[0].slice(data)) != "q" || tokens[0].kind != tokenOperator {
		t.Errorf("token[0] = %q (kind %d), want q operator", tokens[0].slice(data), tokens[0].kind)
	}
	if string(tokens[7].slice(data)) != "cm" || tokens[7].kind != tokenOperator {
		t.Errorf("token[7] = %q (kind %d), want cm operator", tokens[7].slice(data), tokens[7].kind)
	}
	if string(tokens[8].slice(data)) != "Q" || tokens[8].kind != tokenOperator {
		t.Errorf("token[8] = %q, want Q operator", tokens[8].slice(data))
	}
}

func TestScanContentTokens_SkipsCommentsAndStrings(t *testing.T) {
	// A comment containing "cm" must not produce a cm operator token.
	// A literal string containing "cm" must not either.
	data := []byte("BT\n% this is cm in a comment\n(text with cm inside) Tj\nET\n")
	tokens := scanContentTokens(data)

	for _, tok := range tokens {
		if tok.kind == tokenOperator && string(tok.slice(data)) == "cm" {
			t.Errorf("token at [%d:%d] was misclassified as cm operator: %q",
				tok.start, tok.end, tok.slice(data))
		}
	}
	// Verify we still got the real operators.
	gotBT, gotTj, gotET := false, false, false
	for _, tok := range tokens {
		if tok.kind != tokenOperator {
			continue
		}
		switch string(tok.slice(data)) {
		case "BT":
			gotBT = true
		case "Tj":
			gotTj = true
		case "ET":
			gotET = true
		}
	}
	if !gotBT || !gotTj || !gotET {
		t.Errorf("missing operator: BT=%v Tj=%v ET=%v", gotBT, gotTj, gotET)
	}
}

func TestScanContentTokens_HandlesEscapedAndNestedParens(t *testing.T) {
	// PDF strings allow balanced nested parens and escaped delimiters
	// per §7.3.4.2. The lexer must consume the whole literal as one
	// token. The fixture has one paren pair escaped (so it does not
	// affect depth) and an outer balanced pair.
	data := []byte(`(hello \(world\) and more) Tj`)
	tokens := scanContentTokens(data)
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2: %+v", len(tokens), tokens)
	}
	if tokens[0].kind != tokenString {
		t.Errorf("token[0] kind = %d, want string", tokens[0].kind)
	}
	if string(tokens[1].slice(data)) != "Tj" {
		t.Errorf("token[1] = %q, want Tj", tokens[1].slice(data))
	}
}

func TestScanContentTokens_HandlesActualNestedParens(t *testing.T) {
	// Truly nested unescaped parens — depth tracking must produce
	// one string token containing the inner pair.
	data := []byte(`(outer (inner) end) Tj`)
	tokens := scanContentTokens(data)
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2: %+v", len(tokens), tokens)
	}
	if tokens[0].kind != tokenString {
		t.Errorf("token[0] kind = %d, want string", tokens[0].kind)
	}
	if string(tokens[1].slice(data)) != "Tj" {
		t.Errorf("token[1] = %q, want Tj", tokens[1].slice(data))
	}
}

func TestScanContentTokens_HandlesArraysAndDicts(t *testing.T) {
	data := []byte(`[(a) (b) (c)] TJ <</Length 5>> some_op`)
	tokens := scanContentTokens(data)
	if len(tokens) != 4 {
		t.Fatalf("got %d tokens, want 4", len(tokens))
	}
	if tokens[0].kind != tokenArray {
		t.Errorf("token[0] kind = %d, want array", tokens[0].kind)
	}
	if string(tokens[1].slice(data)) != "TJ" {
		t.Errorf("token[1] = %q, want TJ", tokens[1].slice(data))
	}
	if tokens[2].kind != tokenDict {
		t.Errorf("token[2] kind = %d, want dict", tokens[2].kind)
	}
	if string(tokens[3].slice(data)) != "some_op" {
		t.Errorf("token[3] = %q, want some_op", tokens[3].slice(data))
	}
}

func TestScanContentTokens_NumbersWithSignsAndDecimals(t *testing.T) {
	data := []byte("1 -2 3.5 -4.5 .25 +6 m")
	tokens := scanContentTokens(data)
	if len(tokens) != 7 {
		t.Fatalf("got %d tokens, want 7", len(tokens))
	}
	for i := 0; i < 6; i++ {
		if tokens[i].kind != tokenNumber {
			t.Errorf("token[%d] kind = %d, want number; bytes=%q",
				i, tokens[i].kind, tokens[i].slice(data))
		}
	}
}

// --- Pure cleanup function tests ---

func TestCleanContentStreamBytes_DropsIdentityCm(t *testing.T) {
	in := []byte("q\n1 0 0 1 0 0 cm\n100 200 m\nS\nQ\n")
	out := cleanContentStreamBytes(in)
	if bytes.Contains(out, []byte("cm")) {
		t.Errorf("identity cm not removed: %q", out)
	}
	// The drawing operators must be preserved.
	if !bytes.Contains(out, []byte("100 200 m")) {
		t.Errorf("drawing operators lost: %q", out)
	}
}

func TestCleanContentStreamBytes_KeepsNonIdentityCm(t *testing.T) {
	in := []byte("q\n2 0 0 2 0 0 cm\nS\nQ\n")
	out := cleanContentStreamBytes(in)
	if !bytes.Contains(out, []byte("2 0 0 2 0 0 cm")) {
		t.Errorf("non-identity cm was incorrectly removed: %q", out)
	}
}

func TestCleanContentStreamBytes_DropsEmptyQQ(t *testing.T) {
	in := []byte("q\n  \nQ\n100 200 m\nS\n")
	out := cleanContentStreamBytes(in)
	if bytes.Count(out, []byte("q ")) > 0 || bytes.Count(out, []byte("\nq\n")) > 0 {
		t.Errorf("empty q/Q not removed: %q", out)
	}
	// Real ops preserved.
	if !bytes.Contains(out, []byte("100 200 m")) {
		t.Errorf("drawing operators lost: %q", out)
	}
}

func TestCleanContentStreamBytes_KeepsNonEmptyQQ(t *testing.T) {
	in := []byte("q\n100 200 m\nS\nQ\n")
	out := cleanContentStreamBytes(in)
	if !bytes.Contains(out, []byte("q")) || !bytes.Contains(out, []byte("Q")) {
		t.Errorf("non-empty q/Q was removed: %q", out)
	}
	if !bytes.Contains(out, []byte("100 200 m")) {
		t.Errorf("body lost: %q", out)
	}
}

func TestCleanContentStreamBytes_NestedEmptyQQ(t *testing.T) {
	// A `q q Q Q` collapses to nothing in two passes: first pass
	// drops the inner empty pair, then the outer pair becomes empty
	// and is dropped on the second pass.
	in := []byte("q\nq\nQ\nQ\n")
	out := cleanContentStreamBytes(in)
	if bytes.Count(out, []byte("q")) > 0 || bytes.Count(out, []byte("Q")) > 0 {
		t.Errorf("nested empty q/Q not collapsed: %q", out)
	}
}

func TestCleanContentStreamBytes_PreservesContentBetweenIdentityAndDraw(t *testing.T) {
	// Drop identity cm but preserve the surrounding content stream
	// shape — text operators must remain intact.
	in := []byte("q\n1 0 0 1 0 0 cm\nBT\n/F1 12 Tf\n(Hello) Tj\nET\nQ\n")
	out := cleanContentStreamBytes(in)
	// cm gone.
	if bytes.Contains(out, []byte("cm")) {
		t.Errorf("identity cm not removed: %q", out)
	}
	// BT/Tj/ET preserved.
	for _, op := range []string{"BT", "Tj", "ET", "/F1", "12 Tf", "(Hello)"} {
		if !bytes.Contains(out, []byte(op)) {
			t.Errorf("expected %q in output; got %q", op, out)
		}
	}
	// q/Q remain because the body is no longer empty (BT...ET is real work).
	if !bytes.Contains(out, []byte("q")) {
		t.Errorf("q removed despite body containing real work: %q", out)
	}
}

func TestCleanContentStreamBytes_NoChangeOnAlreadyClean(t *testing.T) {
	in := []byte("BT\n/F1 12 Tf\n(Hello) Tj\nET\n")
	out := cleanContentStreamBytes(in)
	if !bytes.Equal(out, in) {
		t.Errorf("clean stream was modified:\n  in  %q\n  out %q", in, out)
	}
}

func TestCleanContentStreamBytes_NoFalsePositivesInsideStrings(t *testing.T) {
	// A string literal containing "1 0 0 1 0 0 cm" or "q Q" must not
	// trigger removal — those are payload bytes, not operators.
	in := []byte("BT\n(this string contains 1 0 0 1 0 0 cm and q Q text) Tj\nET\n")
	out := cleanContentStreamBytes(in)
	if !bytes.Equal(out, in) {
		t.Errorf("string contents triggered removal:\n  in  %q\n  out %q", in, out)
	}
}

func TestCleanContentStreamBytes_IdempotentOnSecondPass(t *testing.T) {
	in := []byte("q\n1 0 0 1 0 0 cm\nq\nQ\n100 200 m\nS\nQ\n")
	first := cleanContentStreamBytes(in)
	second := cleanContentStreamBytes(first)
	if !bytes.Equal(first, second) {
		t.Errorf("second pass mutated already-clean output:\n  first  %q\n  second %q", first, second)
	}
}

// --- Pass-level tests ---

func TestCleanContentStreams_SkipsNonPageStreams(t *testing.T) {
	// A stream not referenced from any Page /Contents must not be
	// touched, even if its bytes look like a content stream.
	w := minimalCatalogWriter(t)
	junk := core.NewPdfStream([]byte("q\n1 0 0 1 0 0 cm\nQ\n"))
	junkRef := w.AddObject(junk)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("Junk", junkRef)
	originalData := append([]byte(nil), junk.Data...)

	w.cleanContentStreams()

	if !bytes.Equal(junk.Data, originalData) {
		t.Error("non-page stream was modified by content cleanup")
	}
}

func TestCleanContentStreams_CleansPageContentsStream(t *testing.T) {
	w := minimalCatalogWriter(t)
	contents := []byte("q\n1 0 0 1 0 0 cm\n100 200 m\nS\nQ\n")
	_, _, contentStream := addPageWithContents(t, w, contents)

	w.cleanContentStreams()

	if bytes.Contains(contentStream.Data, []byte("cm")) {
		t.Errorf("identity cm not removed from page contents: %q", contentStream.Data)
	}
	if !bytes.Contains(contentStream.Data, []byte("100 200 m")) {
		t.Errorf("drawing operators lost: %q", contentStream.Data)
	}
}

func TestCleanContentStreams_HandlesContentsArray(t *testing.T) {
	// A page's /Contents can be an array of indirect references per
	// §7.7.3.3. Each referenced stream must be cleaned.
	w := minimalCatalogWriter(t)
	stream1 := core.NewPdfStream([]byte("1 0 0 1 0 0 cm\n"))
	stream2 := core.NewPdfStream([]byte("q\nQ\n"))
	r1 := w.AddObject(stream1)
	r2 := w.AddObject(stream2)
	page := core.NewPdfDictionary()
	page.Set("Type", core.NewPdfName("Page"))
	page.Set("Contents", core.NewPdfArray(r1, r2))
	pageRef := w.AddObject(page)
	pagesDict := w.objects[1].Object.(*core.PdfDictionary)
	pagesDict.Set("Kids", core.NewPdfArray(pageRef))
	pagesDict.Set("Count", core.NewPdfInteger(1))

	w.cleanContentStreams()

	if bytes.Contains(stream1.Data, []byte("cm")) {
		t.Errorf("stream1 cm not removed: %q", stream1.Data)
	}
	if bytes.Contains(stream2.Data, []byte("q")) || bytes.Contains(stream2.Data, []byte("Q")) {
		t.Errorf("stream2 empty q/Q not removed: %q", stream2.Data)
	}
}

func TestCleanContentStreams_SkipsCompressedStreams(t *testing.T) {
	// Cleanup operates on raw operator text. A stream that is
	// already FlateDecode-compressed must be skipped — inflating is
	// out of scope for this pass.
	w := minimalCatalogWriter(t)
	contents := []byte("q\n1 0 0 1 0 0 cm\nQ\n")
	_, _, contentStream := addPageWithContents(t, w, contents)
	contentStream.Dict.Set("Filter", core.NewPdfName("FlateDecode"))
	originalData := append([]byte(nil), contentStream.Data...)

	w.cleanContentStreams()

	if !bytes.Equal(contentStream.Data, originalData) {
		t.Error("compressed content stream was modified")
	}
}

func TestCleanContentStreams_SizeRegressionGuardRevertsNoOpCleaning(t *testing.T) {
	// A content stream with no removable operators must not be
	// rewritten at all (the size-regression guard reverts an
	// equal-size candidate).
	w := minimalCatalogWriter(t)
	contents := []byte("BT\n/F1 12 Tf\n(Hello) Tj\nET\n")
	_, _, contentStream := addPageWithContents(t, w, contents)
	originalData := append([]byte(nil), contentStream.Data...)

	w.cleanContentStreams()

	if !bytes.Equal(contentStream.Data, originalData) {
		t.Errorf("clean content stream was modified for no benefit:\n  before %q\n  after  %q",
			originalData, contentStream.Data)
	}
}

func TestCleanContentStreams_RefusedOnEncryptedDocument(t *testing.T) {
	w := minimalCatalogWriter(t)
	contents := []byte("q\n1 0 0 1 0 0 cm\nQ\n")
	_, _, contentStream := addPageWithContents(t, w, contents)
	originalData := append([]byte(nil), contentStream.Data...)

	enc, err := core.NewEncryptor(core.RevisionAES128, "user", "owner", core.PermPrint)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	w.SetEncryption(enc)

	var buf bytes.Buffer
	_, err = w.WriteToWithOptions(&buf, WriteOptions{CleanContentStreams: true})
	if err == nil {
		t.Fatal("expected refusal with encryption + CleanContentStreams")
	}
	if !strings.Contains(err.Error(), "content stream cleanup") {
		t.Errorf("error %q does not mention content stream cleanup", err.Error())
	}
	if !bytes.Equal(contentStream.Data, originalData) {
		t.Error("refusal mutated content stream payload")
	}
}

func TestCleanContentStreams_ZeroOptionsByteIdentical(t *testing.T) {
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		addPageWithContents(t, w, []byte("BT (hi) Tj ET\n"))
		return w
	}
	var bufA, bufB bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&bufA, WriteOptions{}); err != nil {
		t.Fatalf("zero opts: %v", err)
	}
	if _, err := makeWriter().WriteTo(&bufB); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if !bytes.Equal(bufA.Bytes(), bufB.Bytes()) {
		t.Error("zero-options output diverges from WriteTo")
	}
}

func TestCleanContentStreams_Idempotent(t *testing.T) {
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		addPageWithContents(t, w, []byte("q\n1 0 0 1 0 0 cm\n100 200 m\nS\nQ\n"))
		return w
	}
	w1 := makeWriter()
	w1.cleanContentStreams()
	var buf1 bytes.Buffer
	if _, err := w1.WriteToWithOptions(&buf1, WriteOptions{}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	w2 := makeWriter()
	w2.cleanContentStreams()
	w2.cleanContentStreams()
	var buf2 bytes.Buffer
	if _, err := w2.WriteToWithOptions(&buf2, WriteOptions{}); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("cleanup not idempotent")
	}
}

func TestCleanContentStreams_ComposesWithRecompress(t *testing.T) {
	// Cleanup runs before recompression; the cleaned (smaller) bytes
	// then get Flate-compressed. Combined output must shrink.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		// Padded with junk so cleanup actually saves visible bytes.
		junk := strings.Repeat("q\n1 0 0 1 0 0 cm\nQ\n", 10)
		body := junk + "BT\n(real text) Tj\nET\n"
		addPageWithContents(t, w, []byte(body))
		return w
	}
	var noCleanup, withCleanup bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&noCleanup, WriteOptions{
		RecompressStreams: true,
	}); err != nil {
		t.Fatalf("recompress only: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&withCleanup, WriteOptions{
		CleanContentStreams: true,
		RecompressStreams:   true,
	}); err != nil {
		t.Fatalf("cleanup + recompress: %v", err)
	}
	if withCleanup.Len() >= noCleanup.Len() {
		t.Errorf("cleanup did not shrink even with recompress: with=%d, without=%d",
			withCleanup.Len(), noCleanup.Len())
	}
}

func TestCleanContentStreams_DocumentAPIDoesNotEnlarge(t *testing.T) {
	doc := buildSampleDocument(5)
	plain, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("plain: %v", err)
	}
	doc2 := buildSampleDocument(5)
	cleaned, err := doc2.ToBytesWithOptions(WriteOptions{CleanContentStreams: true})
	if err != nil {
		t.Fatalf("cleaned: %v", err)
	}
	if len(cleaned) > len(plain) {
		t.Errorf("CleanContentStreams enlarged a layout-built document: plain=%d, cleaned=%d",
			len(plain), len(cleaned))
	}
	if !bytes.HasPrefix(cleaned, []byte("%PDF-")) {
		t.Error("missing PDF header")
	}
	if !bytes.HasSuffix(cleaned, []byte("EOF\n")) {
		t.Error("missing EOF marker")
	}
}

func TestCleanContentStreamBytes_PreservesInlineImageBytes(t *testing.T) {
	// Inline images (BI ... ID <bytes> EI per §8.9.7) carry arbitrary
	// binary payloads that MUST NOT be tokenized as content stream
	// operators. A bug here would let cleanup match patterns inside
	// the image data and silently corrupt it. Construct a payload
	// whose binary bytes happen to contain the strings "q\nQ" and
	// "1 0 0 1 0 0 cm" — both must survive verbatim.
	imageBytes := []byte("\x01\x02q\nQ\x03\x041 0 0 1 0 0 cm\x05\x06")
	in := []byte("q\nBI\n/W 4 /H 4 /CS /G /BPC 8\nID\n")
	in = append(in, imageBytes...)
	in = append(in, []byte("\nEI\nQ\n")...)

	out := cleanContentStreamBytes(in)

	if !bytes.Contains(out, imageBytes) {
		t.Errorf("inline image bytes were corrupted by cleanup\n  in:  %q\n  out: %q",
			in, out)
	}
	// The outer q/Q is empty (only the inline image between them, no
	// real drawing operators). Whether cleanup drops the q/Q is
	// implementation-defined for inline images — but it must not
	// touch the image bytes themselves.
}

func TestCleanContentStreamBytes_HexStringNotMisclassified(t *testing.T) {
	// A hex string `<71 51>` is the ASCII for "qQ" but it is a string
	// literal, not two operators. Cleanup must leave it alone.
	in := []byte("<71 51> Tj\n")
	out := cleanContentStreamBytes(in)
	if !bytes.Equal(out, in) {
		t.Errorf("hex string contents misclassified as operators:\n  in  %q\n  out %q", in, out)
	}
}

func TestCleanContentStreamBytes_EmptyInput(t *testing.T) {
	out := cleanContentStreamBytes(nil)
	if len(out) != 0 {
		t.Errorf("empty input produced %d bytes", len(out))
	}
	out = cleanContentStreamBytes([]byte{})
	if len(out) != 0 {
		t.Errorf("empty slice input produced %d bytes", len(out))
	}
}

func TestCleanContentStreamBytes_AdjacentEmptyPairs(t *testing.T) {
	// Two adjacent `q Q q Q` empty pairs — both must drop in one
	// pass.
	in := []byte("q\nQ\nq\nQ\n")
	out := cleanContentStreamBytes(in)
	if bytes.Contains(out, []byte("q")) || bytes.Contains(out, []byte("Q")) {
		t.Errorf("adjacent empty pairs not collapsed: %q", out)
	}
}

func TestCleanContentStreamBytes_TripleNestedEmptyPairs(t *testing.T) {
	// `q q q Q Q Q` — three levels of empty nesting. The 8-pass cap
	// handles up to 8 levels; 3 is well within that.
	in := []byte("q\nq\nq\nQ\nQ\nQ\n")
	out := cleanContentStreamBytes(in)
	if bytes.Contains(out, []byte("q")) || bytes.Contains(out, []byte("Q")) {
		t.Errorf("triple-nested empty pairs not collapsed: %q", out)
	}
}

func TestCleanContentStreamBytes_MultiLineCm(t *testing.T) {
	// Identity cm with operands separated by newlines instead of
	// spaces. The lexer's whitespace tolerance means the operands
	// still tokenize cleanly; the cleanup should drop them.
	in := []byte("1\n0\n0\n1\n0\n0\ncm\n100 200 m\n")
	out := cleanContentStreamBytes(in)
	if bytes.Contains(out, []byte("cm")) {
		t.Errorf("multi-line identity cm not removed: %q", out)
	}
	if !bytes.Contains(out, []byte("100 200 m")) {
		t.Errorf("drawing op lost after multi-line cm cleanup: %q", out)
	}
}

func TestCleanContentStreamBytes_CommentNotMistakenForOperator(t *testing.T) {
	// A line comment `% ... Q` ending with a Q-shaped string must
	// not produce a Q operator token. A real q above must therefore
	// remain (no matching Q to pair with).
	in := []byte("q\n% comment ending with Q\n100 200 m\nS\nQ\n")
	out := cleanContentStreamBytes(in)
	// Both q and Q must remain because there's a real `100 200 m S`
	// between them.
	if !bytes.Contains(out, []byte("q")) {
		t.Errorf("real q removed: %q", out)
	}
	if !bytes.Contains(out, []byte("Q")) {
		t.Errorf("real Q removed: %q", out)
	}
}

func TestCleanContentStreamBytes_AdversarialUnclosedString(t *testing.T) {
	// A truncated `(unclosed` literal — the lexer must terminate
	// without panicking. The cleanup pass should treat the stream
	// as malformed and either return it unchanged or short-circuit.
	in := []byte("BT (unclosed string ")
	// Just verify no panic and a deterministic result.
	out := cleanContentStreamBytes(in)
	_ = out
}

func TestCleanContentStreamBytes_AdversarialUnclosedHex(t *testing.T) {
	in := []byte("<deadbeef")
	out := cleanContentStreamBytes(in)
	_ = out
}

func TestCleanContentStreamBytes_AdversarialUnclosedDict(t *testing.T) {
	in := []byte("<<unclosed dict")
	out := cleanContentStreamBytes(in)
	_ = out
}

func TestCleanContentStreamBytes_AdversarialUnclosedArray(t *testing.T) {
	in := []byte("[unclosed array")
	out := cleanContentStreamBytes(in)
	_ = out
}

func TestSkipInlineImage_HappyPath(t *testing.T) {
	// Standalone unit on the inline-image skipper.
	data := []byte("BI\n/W 1 /H 1 /BPC 8 /CS /G\nID\nXY\nEI\nq Q")
	tokens := scanContentTokens(data)
	// Expected tokens: BI (operator), q (operator), Q (operator).
	// Everything inside BI..EI must be opaque.
	gotBI, gotQ, gotQQ := false, false, false
	for _, tok := range tokens {
		if tok.kind != tokenOperator {
			continue
		}
		switch string(tok.slice(data)) {
		case "BI":
			gotBI = true
		case "q":
			gotQ = true
		case "Q":
			gotQQ = true
		case "ID", "EI", "XY":
			t.Errorf("unexpected operator %q from inside inline image span: token at [%d:%d]",
				tok.slice(data), tok.start, tok.end)
		}
	}
	if !gotBI || !gotQ || !gotQQ {
		t.Errorf("missing expected operators: BI=%v q=%v Q=%v", gotBI, gotQ, gotQQ)
	}
}

func TestCleanContentStreams_PreservesContentSemantics(t *testing.T) {
	// After cleanup, a content stream that originally drew text
	// must still draw the same text. We verify by checking the
	// post-cleanup bytes still contain the text-positioning and
	// text-showing operators in the right order.
	w := minimalCatalogWriter(t)
	contents := []byte("q\n1 0 0 1 0 0 cm\nBT\n/F1 12 Tf\n100 200 Td\n(Hello, world!) Tj\nET\nQ\n")
	_, _, contentStream := addPageWithContents(t, w, contents)

	w.cleanContentStreams()

	// All semantic operators preserved in original order.
	idx := 0
	for _, op := range []string{"BT", "/F1", "Tf", "100 200 Td", "Hello, world!", "Tj", "ET"} {
		next := bytes.Index(contentStream.Data[idx:], []byte(op))
		if next < 0 {
			t.Errorf("operator %q missing or out of order in cleaned output: %q",
				op, contentStream.Data)
			return
		}
		idx += next + len(op)
	}
}
