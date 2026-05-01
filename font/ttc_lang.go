// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"slices"
	"strings"
)

// pickFaceForLanguage scans a TrueType Collection's faces, reads each
// face's NameID 1 (FontFamily) from its `name` table, and returns the
// index of the face whose family name best matches lang. Returns -1
// when no face matches; the caller falls back to face 0 (the
// browser-style default for url() references without a `#` fragment).
//
// lang is a BCP-47 language tag like "zh-CN", "ja", "ko", or a region
// hint like "zh", "Hant", "Hans". Empty string skips matching and
// returns -1 immediately. Matching is substring-based against the
// regional tokens that pan-CJK font collections embed in their
// family names — "Noto Sans CJK JP", "Source Han Sans SC", etc. —
// rather than a full BCP-47 resolver, because that's the level of
// signal these fonts actually carry.
//
// Spec context: ISO/IEC 14496-22 §5.2.5 NameID 1 = FontFamily. TTCs
// repeat the name-table per face, so each face advertises its own
// regional identity.
func pickFaceForLanguage(data []byte, lang string) int {
	tokens := languageTokens(lang)
	if len(tokens) == 0 {
		return -1
	}
	dataLen := uint64(len(data))
	if dataLen < 12 {
		return -1
	}
	if binary.BigEndian.Uint32(data[0:4]) != ttcMagic {
		return -1
	}
	numFonts := uint64(binary.BigEndian.Uint32(data[8:12]))
	if numFonts == 0 || dataLen < 12+numFonts*4 {
		return -1
	}

	for i := uint64(0); i < numFonts; i++ {
		fontOff := uint64(binary.BigEndian.Uint32(data[12+i*4:]))
		family := readFontFamily(data, fontOff)
		if family == "" {
			continue
		}
		if familyContainsAnyToken(family, tokens) {
			return int(i)
		}
	}
	return -1
}

// familyContainsAnyToken returns true when family — split into
// space-separated words — contains at least one of tokens as a
// whole word. Whole-word matching avoids the false positives
// [strings.Contains] would produce against family names like
// "Noto Sans Special Compressed" (contains the substring "SC" in
// "Special" / "Compressed") or "JapanesEsque Title" — neither of
// which represents a Japanese-targeting face. Pan-CJK collections
// embed tokens as whole words ("Noto Sans CJK JP"), so the tighter
// match still hits everything we need to.
//
// Comparison is case-sensitive because the regional tokens we
// match against are stable upper-case ("JP", "SC") and the family
// names that carry them are also case-stable. A future need for
// case-insensitive matching would lower-case both sides.
func familyContainsAnyToken(family string, tokens []string) bool {
	for _, word := range strings.Fields(family) {
		if slices.Contains(tokens, word) {
			return true
		}
	}
	return false
}

// readFontFamily extracts the FontFamily (NameID 1) string for a
// single face starting at fontOff in a TTC. Returns the decoded
// string, or "" when the face has no name table or no readable
// FontFamily record. Reuses [parseName]'s scoring logic but filters
// to NameID 1 instead of 4/6 — the regional identity lives in the
// FontFamily record across every CJK collection we tested
// (NotoSansCJK, Source Han Sans, Hiragino, PingFang).
func readFontFamily(data []byte, fontOff uint64) string {
	dataLen := uint64(len(data))
	if fontOff+12 > dataLen {
		return ""
	}
	numTables := uint64(binary.BigEndian.Uint16(data[fontOff+4:]))
	if fontOff+12+numTables*16 > dataLen {
		return ""
	}
	var nameOff, nameLen uint64
	for i := uint64(0); i < numTables; i++ {
		entry := fontOff + 12 + i*16
		if string(data[entry:entry+4]) != "name" {
			continue
		}
		nameOff = uint64(binary.BigEndian.Uint32(data[entry+8:]))
		nameLen = uint64(binary.BigEndian.Uint32(data[entry+12:]))
		break
	}
	if nameOff == 0 || nameOff+nameLen > dataLen {
		return ""
	}
	return readNameID1(data[nameOff : nameOff+nameLen])
}

// readNameID1 decodes the FontFamily (NameID 1) string from a
// `name` table buffer. Mirrors the Windows-Unicode/Mac-Roman fallback
// scoring used by [parseName] for NameIDs 4 and 6, but parameterised
// to NameID 1 so the TTC face picker doesn't have to consult the
// general-purpose name parser.
func readNameID1(name []byte) string {
	dataLen := uint64(len(name))
	if dataLen < 6 {
		return ""
	}
	count := uint64(binary.BigEndian.Uint16(name[2:4]))
	stringOff := uint64(binary.BigEndian.Uint16(name[4:6]))
	if 6+count*12 > dataLen || stringOff > dataLen {
		return ""
	}
	bestScore := 0
	var bestData []byte
	var bestDec func([]byte) string
	for i := uint64(0); i < count; i++ {
		off := 6 + i*12
		platformID := binary.BigEndian.Uint16(name[off : off+2])
		encodingID := binary.BigEndian.Uint16(name[off+2 : off+4])
		nameID := binary.BigEndian.Uint16(name[off+6 : off+8])
		length := uint64(binary.BigEndian.Uint16(name[off+8 : off+10]))
		recOff := uint64(binary.BigEndian.Uint16(name[off+10 : off+12]))
		if nameID != 1 {
			continue
		}
		score, dec := scoreNameEncoding(platformID, encodingID)
		if score == 0 {
			continue
		}
		strStart := stringOff + recOff
		strEnd := strStart + length
		if strEnd > dataLen {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestData = name[strStart:strEnd]
			bestDec = dec
		}
	}
	if bestDec == nil {
		return ""
	}
	return bestDec(bestData)
}

// languageTokens returns the regional substring tokens to search for
// in face FontFamily strings, given a BCP-47-ish language tag. The
// tokens are deliberately specific to pan-CJK collections because
// that's the only family of font collections in the wild whose face
// names encode regional variants — Latin font collections (Helvetica,
// Source Sans, etc.) use weight tokens (Bold, Light) instead, which
// callers should select via the existing weight/style API rather than
// language.
//
// Returns an empty slice for empty input or for languages that don't
// have a CJK-collection convention (the picker falls back to face 0).
func languageTokens(lang string) []string {
	if lang == "" {
		return nil
	}
	l := strings.ToLower(lang)
	switch {
	case strings.HasPrefix(l, "zh-hans"), strings.HasPrefix(l, "zh-cn"),
		strings.HasPrefix(l, "zh-sg"):
		return []string{"SC", "Hans", "Simplified"}
	case strings.HasPrefix(l, "zh-hant"), strings.HasPrefix(l, "zh-tw"),
		strings.HasPrefix(l, "zh-hk"), strings.HasPrefix(l, "zh-mo"):
		return []string{"TC", "Hant", "Traditional"}
	case strings.HasPrefix(l, "zh"):
		// Bare "zh" without region defaults to Simplified. RFC 5646
		// itself takes no position on default scripts; this matches
		// CLDR's likelySubtags entry mapping `zh` → `zh_Hans_CN`,
		// which is what browsers (Chrome, Safari) and Wikipedia use
		// when only `zh` is supplied. Callers who need the legacy
		// `zh` → Traditional default should pass `zh-TW` or
		// `zh-Hant` explicitly.
		return []string{"SC", "Hans", "Simplified"}
	case strings.HasPrefix(l, "ja"):
		return []string{"JP", "Japanese", "Jpan"}
	case strings.HasPrefix(l, "ko"):
		return []string{"KR", "Korean", "Kore"}
	}
	return nil
}
