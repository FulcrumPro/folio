// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// testCFFFontPath returns a path to an OTF (CFF-flavored) font that
// exists on the test system, or skips the test. Real CFF outlines are
// needed because the dispatch is gated on the presence of the `CFF `
// table; a synthetic minimal sfnt would still exercise the same code
// path but at the cost of hand-rolling head/hhea/maxp/hmtx/cmap. Once
// the Phase 2 parser lands we can switch these tests to a synthetic
// fixture and drop the system-font dependency.
func testCFFFontPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/System/Library/Fonts/Supplemental/STIXGeneral.otf",
		"/System/Library/Fonts/Supplemental/NotoSansCanadianAboriginal-Regular.otf",
		"/System/Library/Fonts/LastResort.otf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no CFF/OTF font available on this system")
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

func TestIsCFFDetectsOutlineFormat(t *testing.T) {
	cff := loadTestCFFFace(t)
	cf, ok := cff.(cffFace)
	if !ok {
		t.Fatalf("CFF face does not implement cffFace")
	}
	if !cf.IsCFF() {
		t.Error("expected IsCFF() = true for OTF/CFF font")
	}
	if len(cf.CFFData()) == 0 {
		t.Error("expected CFFData() to return non-empty bytes for CFF font")
	}

	ttf := loadTestFace(t)
	cf2, ok := ttf.(cffFace)
	if !ok {
		t.Fatalf("TTF face does not implement cffFace")
	}
	if cf2.IsCFF() {
		t.Error("expected IsCFF() = false for TrueType font")
	}
	if cf2.CFFData() != nil {
		t.Error("expected CFFData() = nil for TrueType font")
	}
}

func TestFaceCFFDataHelper(t *testing.T) {
	// TrueType: helper returns (nil, false).
	ttf := loadTestFace(t)
	if data, ok := faceCFFData(ttf); ok || data != nil {
		t.Errorf("faceCFFData(TTF) = (%d bytes, %v), want (nil, false)", len(data), ok)
	}

	// CFF: helper returns (bytes, true) with the same payload as
	// CFFData() directly. We compare lengths rather than full bytes
	// to keep the assertion meaningful without ballooning test
	// memory for large CJK fonts.
	cff := loadTestCFFFace(t)
	data, ok := faceCFFData(cff)
	if !ok {
		t.Fatal("faceCFFData(CFF) returned ok=false")
	}
	if len(data) == 0 {
		t.Error("faceCFFData(CFF) returned empty bytes")
	}
	if direct := cff.(cffFace).CFFData(); len(direct) != len(data) {
		t.Errorf("faceCFFData length %d != CFFData length %d", len(data), len(direct))
	}
}

// TestBuildObjectsCFFDispatch exercises the embed-path branch added in
// Phase 1: a CFF face must produce a /FontFile3 stream with
// /CIDFontType0C subtype, a /CIDFontType0 descendant font, no
// /CIDToGIDMap, and none of the TrueType-only keys (/FontFile2,
// /CIDFontType2, /Length1).
func TestBuildObjectsCFFDispatch(t *testing.T) {
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

	// CFF path emits the same four indirect objects as TrueType:
	// font stream, descriptor, CIDFont, ToUnicode.
	if len(objects) != 4 {
		t.Fatalf("expected 4 indirect objects, got %d", len(objects))
	}

	// Serialize every object so we can grep for keys without
	// asserting on indirect-reference numbers.
	dump := func(o core.PdfObject) string {
		var buf bytes.Buffer
		_, _ = o.WriteTo(&buf)
		return buf.String()
	}
	streamText := dump(objects[0])
	descriptorText := dump(objects[1])
	cidFontText := dump(objects[2])
	type0Text := dump(type0)

	// Font stream must declare CFF subtype, never carry /Length1.
	if !strings.Contains(streamText, "/Subtype /CIDFontType0C") {
		t.Errorf("font stream missing /Subtype /CIDFontType0C:\n%s", streamText)
	}
	if strings.Contains(streamText, "/Length1") {
		t.Error("font stream must not carry /Length1 in CFF path")
	}

	// FontDescriptor uses /FontFile3, not /FontFile2.
	if !strings.Contains(descriptorText, "/FontFile3") {
		t.Errorf("descriptor missing /FontFile3:\n%s", descriptorText)
	}
	if strings.Contains(descriptorText, "/FontFile2") {
		t.Error("descriptor must not reference /FontFile2 in CFF path")
	}

	// Descendant CIDFont must be CIDFontType0 and must NOT declare
	// CIDToGIDMap (/Identity is implicit for /CIDFontType0; spec
	// readers reject the explicit key).
	if !strings.Contains(cidFontText, "/Subtype /CIDFontType0") {
		t.Errorf("CIDFont missing /Subtype /CIDFontType0:\n%s", cidFontText)
	}
	if strings.Contains(cidFontText, "/CIDFontType2") {
		t.Error("CIDFont must not declare /CIDFontType2 in CFF path")
	}
	if strings.Contains(cidFontText, "/CIDToGIDMap") {
		t.Error("CIDFont must omit /CIDToGIDMap for /CIDFontType0")
	}

	// Shared behaviour with TrueType path: Type0 wrapper, Identity-H,
	// ToUnicode link.
	if !strings.Contains(type0Text, "/Subtype /Type0") {
		t.Errorf("Type0 dict missing /Subtype /Type0:\n%s", type0Text)
	}
	if !strings.Contains(type0Text, "/Encoding /Identity-H") {
		t.Errorf("Type0 dict missing /Encoding /Identity-H:\n%s", type0Text)
	}
	if !strings.Contains(type0Text, "/ToUnicode") {
		t.Errorf("Type0 dict missing /ToUnicode:\n%s", type0Text)
	}
}

// TestBuildObjectsTrueTypeUnchanged guards against the dispatch
// inadvertently rerouting TrueType faces through the CFF path. This
// is a regression test for the embed.go change in Phase 1.
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

	var descriptorBuf bytes.Buffer
	_, _ = objects[1].WriteTo(&descriptorBuf)
	descriptorText := descriptorBuf.String()
	if !strings.Contains(descriptorText, "/FontFile2") {
		t.Errorf("TrueType descriptor must still use /FontFile2:\n%s", descriptorText)
	}
	if strings.Contains(descriptorText, "/FontFile3") {
		t.Error("TrueType descriptor must not switch to /FontFile3")
	}

	var cidBuf bytes.Buffer
	_, _ = objects[2].WriteTo(&cidBuf)
	cidText := cidBuf.String()
	if !strings.Contains(cidText, "/Subtype /CIDFontType2") {
		t.Errorf("TrueType CIDFont must remain /CIDFontType2:\n%s", cidText)
	}
	if !strings.Contains(cidText, "/CIDToGIDMap /Identity") {
		t.Errorf("TrueType CIDFont must keep /CIDToGIDMap /Identity:\n%s", cidText)
	}
}
