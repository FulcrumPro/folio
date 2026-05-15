// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"os"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// testCFFFontPath returns a path to a CID-keyed CFF font (the dispatch
// the embedder now gates on) that exists on the test system, or skips
// the test. Non-CID-keyed CFF fonts are explicitly excluded here: they
// fall through to the legacy TrueType embed path by design, so testing
// the CFF dispatch with one of them would mis-fire the assertions.
//
// A synthetic sfnt-with-CFF fixture would remove the system-font
// dependency entirely; that lands with the Phase 2 CFF parser, which
// makes hand-rolling a valid sfnt-wrapped CID-keyed CFF a side-effect
// of work already in progress rather than a one-off test fixture.
func testCFFFontPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/System/Library/Fonts/Hiragino Sans GB.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no CID-keyed CFF font available on this system")
	return ""
}

// testNonCIDKeyedCFFFontPath returns a Latin/.otf CFF font (name-keyed)
// for testing that the dispatcher correctly *excludes* it. STIXGeneral
// is widely available on macOS; on Linux there's no comparably
// universal name-keyed CFF, so the test skips.
func testNonCIDKeyedCFFFontPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/System/Library/Fonts/Supplemental/STIXGeneral.otf",
		"/System/Library/Fonts/Supplemental/NotoSansCanadianAboriginal-Regular.otf",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no name-keyed CFF font available on this system")
	return ""
}

func loadTestCFFFace(t *testing.T) Face {
	t.Helper()
	face, err := LoadFont(testCFFFontPath(t))
	if err != nil {
		t.Fatalf("LoadFont CFF: %v", err)
	}
	return face
}

func TestIsCFFAndCFF2DetectOutlineFormat(t *testing.T) {
	cff := loadTestCFFFace(t)
	cf, ok := cff.(cffFace)
	if !ok {
		t.Fatalf("CFF face does not implement cffFace")
	}
	if !cf.IsCFF() {
		t.Error("expected IsCFF() = true for OTF/CFF font")
	}
	if cf.IsCFF2() {
		t.Error("IsCFF2() must be false for a plain CFF v1 font")
	}
	if len(cf.CFFData()) == 0 {
		t.Error("CFFData() returned empty for CFF font")
	}

	ttf := loadTestFace(t)
	cf2, ok := ttf.(cffFace)
	if !ok {
		t.Fatalf("TTF face does not implement cffFace")
	}
	if cf2.IsCFF() || cf2.IsCFF2() {
		t.Error("TrueType face must have both IsCFF/IsCFF2 false")
	}
	if cf2.CFFData() != nil {
		t.Error("CFFData() must be nil for TrueType")
	}
}

func TestFaceCFFDataAcceptsCIDKeyed(t *testing.T) {
	face := loadTestCFFFace(t)
	data, ok := faceCFFData(face)
	if !ok {
		t.Fatal("faceCFFData rejected CID-keyed CFF face")
	}
	if len(data) == 0 {
		t.Error("faceCFFData returned empty bytes for CID-keyed CFF")
	}
}

// TestFaceCFFDataExcludesNameKeyed verifies the CID-keyed gate added
// in Phase 1.5. A plain Latin CFF font has a `CFF ` table but the Top
// DICT does not open with ROS, so the dispatcher must keep it on the
// legacy embed path until a non-CIDFont CFF graph (/Type1C) lands.
func TestFaceCFFDataExcludesNameKeyed(t *testing.T) {
	path := testNonCIDKeyedCFFFontPath(t)
	face, err := LoadFont(path)
	if err != nil {
		t.Fatalf("LoadFont %s: %v", path, err)
	}
	cf := face.(cffFace)
	if !cf.IsCFF() {
		t.Fatalf("test precondition broken: %s should be CFF", path)
	}
	if _, ok := faceCFFData(face); ok {
		t.Errorf("faceCFFData accepted name-keyed CFF; expected (nil, false)")
	}
}

func TestFaceCFFDataRejectsTrueType(t *testing.T) {
	if data, ok := faceCFFData(loadTestFace(t)); ok || data != nil {
		t.Errorf("faceCFFData(TTF) = (%d bytes, %v), want (nil, false)", len(data), ok)
	}
}

// name extracts the value of a /Name PDF object from a dictionary
// slot. The two failures (key absent, value not a name) are folded
// into a single error path because test callers only need the name
// string or "missing".
func name(t *testing.T, d *core.PdfDictionary, key string) string {
	t.Helper()
	v := d.Get(key)
	if v == nil {
		return ""
	}
	n, ok := v.(*core.PdfName)
	if !ok {
		t.Fatalf("dict key %q: expected *PdfName, got %T", key, v)
	}
	return n.Value
}

func hasKey(d *core.PdfDictionary, key string) bool {
	return d.Get(key) != nil
}

// TestBuildObjectsCFFDispatchStructural asserts the CFF embed object
// graph by reading PDF dictionary keys directly. Substring matches on
// the serialized PDF text are fragile — `/CIDFontType0` is a strict
// prefix of `/CIDFontType0C`, so a regression that wrote the wrong
// subtype on the wrong dict would not be caught by a grep-based
// assertion.
func TestBuildObjectsCFFDispatchStructural(t *testing.T) {
	face := loadTestCFFFace(t)
	ef := NewEmbeddedFont(face)
	ef.EncodeString("Test")

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		n := len(objects) + 1
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(n, 0)
	}
	type0 := ef.BuildObjects(addObject)

	if len(objects) != 4 {
		t.Fatalf("want 4 indirect objects, got %d", len(objects))
	}

	// Object 0: the CFF stream. Subtype lives on the stream dict.
	stream, ok := objects[0].(*core.PdfStream)
	if !ok {
		t.Fatalf("objects[0] is %T, want *PdfStream", objects[0])
	}
	if got := name(t, stream.Dict, "Subtype"); got != "CIDFontType0C" {
		t.Errorf("stream /Subtype = %q, want CIDFontType0C", got)
	}
	if hasKey(stream.Dict, "Length1") {
		t.Error("stream must not carry /Length1 on the CFF path")
	}

	// Object 1: FontDescriptor.
	descriptor, ok := objects[1].(*core.PdfDictionary)
	if !ok {
		t.Fatalf("objects[1] is %T, want *PdfDictionary", objects[1])
	}
	if got := name(t, descriptor, "Type"); got != "FontDescriptor" {
		t.Errorf("descriptor /Type = %q", got)
	}
	if !hasKey(descriptor, "FontFile3") {
		t.Error("descriptor missing /FontFile3")
	}
	if hasKey(descriptor, "FontFile2") {
		t.Error("descriptor must not carry /FontFile2 on CFF path")
	}
	for _, k := range []string{"FontName", "Flags", "FontBBox", "Ascent", "Descent", "CapHeight", "StemV", "ItalicAngle"} {
		if !hasKey(descriptor, k) {
			t.Errorf("descriptor missing /%s", k)
		}
	}

	// Object 2: CIDFont (descendant of Type0).
	cidFont, ok := objects[2].(*core.PdfDictionary)
	if !ok {
		t.Fatalf("objects[2] is %T, want *PdfDictionary", objects[2])
	}
	if got := name(t, cidFont, "Subtype"); got != "CIDFontType0" {
		t.Errorf("CIDFont /Subtype = %q, want exactly CIDFontType0", got)
	}
	if got := name(t, cidFont, "Type"); got != "Font" {
		t.Errorf("CIDFont /Type = %q", got)
	}
	if hasKey(cidFont, "CIDToGIDMap") {
		t.Error("CIDFont must omit /CIDToGIDMap for /CIDFontType0")
	}
	for _, k := range []string{"BaseFont", "CIDSystemInfo", "FontDescriptor", "DW", "W"} {
		if !hasKey(cidFont, k) {
			t.Errorf("CIDFont missing /%s", k)
		}
	}

	// Type0 wrapper.
	if got := name(t, type0, "Subtype"); got != "Type0" {
		t.Errorf("Type0 /Subtype = %q", got)
	}
	if got := name(t, type0, "Encoding"); got != "Identity-H" {
		t.Errorf("Type0 /Encoding = %q", got)
	}
	for _, k := range []string{"BaseFont", "DescendantFonts", "ToUnicode"} {
		if !hasKey(type0, k) {
			t.Errorf("Type0 missing /%s", k)
		}
	}
}

// TestBuildObjectsNameKeyedCFFFallthrough confirms a plain Latin CFF
// (no ROS) takes the legacy TrueType embed path. This is a behavior
// regression test for the gate: it must NOT silently route to the
// CFF path and emit the wrong stream subtype.
func TestBuildObjectsNameKeyedCFFFallthrough(t *testing.T) {
	path := testNonCIDKeyedCFFFontPath(t)
	face, err := LoadFont(path)
	if err != nil {
		t.Fatalf("LoadFont %s: %v", path, err)
	}
	ef := NewEmbeddedFont(face)
	ef.EncodeString("Test")

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		n := len(objects) + 1
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(n, 0)
	}
	ef.BuildObjects(addObject)

	descriptor, ok := objects[1].(*core.PdfDictionary)
	if !ok {
		t.Fatalf("objects[1] is %T", objects[1])
	}
	// Legacy path: /FontFile2, not /FontFile3.
	if !hasKey(descriptor, "FontFile2") {
		t.Error("name-keyed CFF should fall through to /FontFile2 path")
	}
	if hasKey(descriptor, "FontFile3") {
		t.Error("name-keyed CFF must not take /FontFile3 path until non-CID CFF support lands")
	}

	cidFont := objects[2].(*core.PdfDictionary)
	if got := name(t, cidFont, "Subtype"); got != "CIDFontType2" {
		t.Errorf("name-keyed CFF /Subtype = %q, want CIDFontType2 on legacy path", got)
	}
}

// TestBuildObjectsTrueTypeUnchanged guards against the dispatch
// inadvertently rerouting TrueType faces through the CFF path.
func TestBuildObjectsTrueTypeUnchanged(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)
	ef.EncodeString("Test")

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		n := len(objects) + 1
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(n, 0)
	}
	ef.BuildObjects(addObject)

	descriptor := objects[1].(*core.PdfDictionary)
	if !hasKey(descriptor, "FontFile2") {
		t.Error("TrueType descriptor must still use /FontFile2")
	}
	if hasKey(descriptor, "FontFile3") {
		t.Error("TrueType descriptor must not switch to /FontFile3")
	}

	cidFont := objects[2].(*core.PdfDictionary)
	if got := name(t, cidFont, "Subtype"); got != "CIDFontType2" {
		t.Errorf("TrueType CIDFont /Subtype = %q, want CIDFontType2", got)
	}
	if got := name(t, cidFont, "CIDToGIDMap"); got != "Identity" {
		t.Errorf("TrueType CIDFont /CIDToGIDMap = %q, want Identity", got)
	}
}

// TestBuildObjectsCFFSubsetsFontStream is the embed-path regression
// test for issue #295. A CFF face passed through BuildObjects after
// encoding a single CJK character must produce a font stream whose
// raw payload is dramatically smaller than the source CFF table —
// confirming SubsetCFF is wired in. The Phase 1 unsubsetted path
// produced ~14 MB for this scenario; subset bytes should be well
// under 1 MB.
func TestBuildObjectsCFFSubsetsFontStream(t *testing.T) {
	face := loadTestCFFFace(t)
	cf := face.(cffFace)
	srcLen := len(cf.CFFData())

	ef := NewEmbeddedFont(face)
	ef.EncodeString("a") // single CJK or Latin glyph — content doesn't matter

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		n := len(objects) + 1
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(n, 0)
	}
	ef.BuildObjects(addObject)

	stream := objects[0].(*core.PdfStream)
	if len(stream.Data) >= srcLen/10 {
		t.Errorf("embedded font stream size %d not significantly smaller than source CFF %d",
			len(stream.Data), srcLen)
	}
	t.Logf("CFF font stream embed: %d -> %d bytes (%.1f%%)",
		srcLen, len(stream.Data), float64(len(stream.Data))*100/float64(srcLen))
}

// TestBuildObjectsCFFSubsetTagPrefix verifies that when SubsetCFF
// succeeds, the six-letter "XXXXXX+" subset tag prefix is applied
// consistently to /BaseFont (Type0 and CIDFont) and /FontName
// (descriptor) per ISO 32000-1 §9.6.4. PDF readers (Acrobat in
// particular) warn when these disagree.
func TestBuildObjectsCFFSubsetTagPrefix(t *testing.T) {
	face := loadTestCFFFace(t)
	ef := NewEmbeddedFont(face)
	ef.EncodeString("a")

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		n := len(objects) + 1
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(n, 0)
	}
	type0 := ef.BuildObjects(addObject)

	prefixOf := func(d *core.PdfDictionary, key string) string {
		s := name(t, d, key)
		if len(s) < 7 || s[6] != '+' {
			return ""
		}
		return s[:6]
	}
	type0Tag := prefixOf(type0, "BaseFont")
	cidTag := prefixOf(objects[2].(*core.PdfDictionary), "BaseFont")
	descTag := prefixOf(objects[1].(*core.PdfDictionary), "FontName")
	if type0Tag == "" || type0Tag != cidTag || type0Tag != descTag {
		t.Errorf("subset tag mismatch: Type0=%q CIDFont=%q Descriptor=%q",
			type0Tag, cidTag, descTag)
	}
}

// TestBuildObjectsCFFEmptyUsedGlyphs covers the edge case where no
// EncodeString call has happened. The CFF builder must still emit four
// indirect objects with sensible defaults (empty /W array, empty
// ToUnicode bfchar block).
func TestBuildObjectsCFFEmptyUsedGlyphs(t *testing.T) {
	face := loadTestCFFFace(t)
	ef := NewEmbeddedFont(face)

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		n := len(objects) + 1
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(n, 0)
	}
	type0 := ef.BuildObjects(addObject)

	if len(objects) != 4 {
		t.Fatalf("want 4 indirect objects, got %d", len(objects))
	}
	cidFont := objects[2].(*core.PdfDictionary)
	if !hasKey(cidFont, "W") {
		t.Error("/W must be present even when empty")
	}
	if got := name(t, type0, "Subtype"); got != "Type0" {
		t.Errorf("Type0 /Subtype = %q", got)
	}
}
