// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"maps"

	"github.com/carlos7ags/folio/core"
)

// buildObjectsCFF builds the PDF object graph for a CFF-flavored
// embedded font. The structure mirrors the TrueType path in
// [EmbeddedFont.BuildObjects] but differs in three places, per ISO
// 32000-1 §9.8 (font descriptors) and §9.7 (CID fonts):
//
//  1. The font stream is `/FontFile3` with `/Subtype /CIDFontType0C`
//     and no `/Length1` (Length1 applies only to Type 1 / TrueType
//     streams).
//  2. The descendant CIDFont has `/Subtype /CIDFontType0` (CFF
//     outlines) rather than `/CIDFontType2` (TrueType outlines).
//  3. `/CIDToGIDMap` is omitted: CFF's intrinsic charset already maps
//     CIDs to glyph slots, so the explicit GID map is meaningless and
//     PDF readers reject it on /CIDFontType0.
//
// `/W`, the ToUnicode CMap, the Type0 wrapper, `/Encoding /Identity-H`,
// and the `Adobe-Identity-0` ordering are reused unchanged from the
// TrueType path so that text shaping, copy/paste, and accessibility
// behave identically across outline formats.
//
// During Phase 1 the CFF bytes are embedded unmodified — equivalent to
// the existing fallback behavior of [Subset] for unsupported formats,
// but with the correct PDF object graph. Phase 3+ replaces the
// `cffData` payload with a subset CFF blob.
func (ef *EmbeddedFont) buildObjectsCFF(cffData []byte, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfDictionary {
	face := ef.face
	psName := sanitizePSName(face.PostScriptName())
	upem := face.UnitsPerEm()

	// Always include .notdef (GID 0) in the used-glyph set; the
	// subset tag in the PostScript name is derived from this set.
	glyphs := make(map[uint16]rune, len(ef.usedGlyphs)+1)
	glyphs[0] = 0
	maps.Copy(glyphs, ef.usedGlyphs)

	// Phase 1: embed CFF bytes unchanged. Phase 3+ will replace this
	// call with SubsetCFF(cffData, glyphs) and apply the subset tag
	// prefix to psName on success.
	fontStream := core.NewPdfStreamCompressed(cffData)
	fontStream.Dict.Set("Subtype", core.NewPdfName("CIDFontType0C"))
	fontStreamRef := addObject(fontStream)

	descriptor := buildCFFFontDescriptor(face, psName, fontStreamRef)
	descriptorRef := addObject(descriptor)

	cidFont := core.NewPdfDictionary()
	cidFont.Set("Type", core.NewPdfName("Font"))
	cidFont.Set("Subtype", core.NewPdfName("CIDFontType0"))
	cidFont.Set("BaseFont", core.NewPdfName(psName))
	cidFont.Set("CIDSystemInfo", buildCIDSystemInfo())
	cidFont.Set("FontDescriptor", descriptorRef)
	cidFont.Set("DW", core.NewPdfInteger(1000))
	cidFont.Set("W", buildWidthArray(ef, upem))
	// /CIDToGIDMap is intentionally omitted for /CIDFontType0; CFF
	// charset provides the CID-to-glyph mapping intrinsically.
	cidFontRef := addObject(cidFont)

	toUnicode := core.NewPdfStreamCompressed([]byte(ef.buildToUnicodeCMap()))
	toUnicodeRef := addObject(toUnicode)

	type0 := core.NewPdfDictionary()
	type0.Set("Type", core.NewPdfName("Font"))
	type0.Set("Subtype", core.NewPdfName("Type0"))
	type0.Set("BaseFont", core.NewPdfName(psName))
	type0.Set("Encoding", core.NewPdfName("Identity-H"))
	type0.Set("DescendantFonts", core.NewPdfArray(cidFontRef))
	type0.Set("ToUnicode", toUnicodeRef)

	return type0
}

// buildCFFFontDescriptor assembles the /FontDescriptor for a CFF-
// flavored CIDFont. The only structural difference from the TrueType
// descriptor is the use of /FontFile3 in place of /FontFile2.
func buildCFFFontDescriptor(face Face, psName string, fontStreamRef *core.PdfIndirectReference) *core.PdfDictionary {
	bbox := face.BBox()
	d := core.NewPdfDictionary()
	d.Set("Type", core.NewPdfName("FontDescriptor"))
	d.Set("FontName", core.NewPdfName(psName))
	d.Set("Flags", core.NewPdfInteger(int(face.Flags())))
	d.Set("FontBBox", core.NewPdfArray(
		core.NewPdfInteger(bbox[0]),
		core.NewPdfInteger(bbox[1]),
		core.NewPdfInteger(bbox[2]),
		core.NewPdfInteger(bbox[3]),
	))
	d.Set("ItalicAngle", core.NewPdfReal(face.ItalicAngle()))
	d.Set("Ascent", core.NewPdfInteger(face.Ascent()))
	d.Set("Descent", core.NewPdfInteger(face.Descent()))
	capHeight := face.CapHeight()
	if capHeight == 0 {
		capHeight = face.Ascent()
	}
	d.Set("CapHeight", core.NewPdfInteger(capHeight))
	stemV := face.StemV()
	if stemV == 0 {
		stemV = 80
	}
	d.Set("StemV", core.NewPdfInteger(stemV))
	d.Set("FontFile3", fontStreamRef)
	return d
}
