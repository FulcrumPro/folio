// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
	"os"
)

// ParseFont parses a font from raw bytes, auto-detecting the format.
// Supports TTF, OTF, WOFF1, and TrueType Collection (TTC) fonts. For
// collections, the first face is selected — matching the convention used
// by browsers and font tools for url() references without a `#` fragment.
//
// Programmatic callers who need a non-default face from a TTC (for
// example, the SC variant of NotoSansCJK rather than the JP variant
// at face 0) should use [ParseFontForLanguage] instead.
//
// Errors returned by this function wrap one of the sentinel errors
// [ErrUnknownFormat], [ErrTruncated], or [ErrCorruptTable] so callers
// can match failure modes with errors.Is.
func ParseFont(data []byte) (Face, error) {
	return ParseFontForLanguage(data, "")
}

// ParseFontForLanguage parses a font from raw bytes like [ParseFont]
// but, for TrueType Collections, picks the face whose name-table
// FontFamily best matches the given BCP-47 language tag. Empty lang
// (or any non-TTC input) behaves identically to ParseFont — face 0
// for collections, the single face for standalone TTF/OTF/WOFF.
//
// Pan-CJK font collections (NotoSansCJK, Source Han Sans, Hiragino,
// PingFang, msgothic.ttc, Apple's STHeiti family) ship with separate
// faces for Japanese, Korean, Simplified Chinese, and Traditional
// Chinese variants. Their FontFamily names embed regional tokens
// like "JP", "SC", "TC", "Hans", "Hant", "Japanese", "Korean". This
// function matches those tokens against the requested language; a
// match returns the corresponding face, no match falls back to face
// 0 (so existing behavior is unchanged when the hint is absent or
// doesn't apply).
//
// Recognised language hints:
//
//   - "zh-CN", "zh-Hans", "zh-SG" — Simplified Chinese (SC / Hans)
//   - "zh-TW", "zh-Hant", "zh-HK", "zh-MO" — Traditional Chinese (TC / Hant)
//   - "zh" alone — Simplified, matching the IETF/CLDR default-script
//   - "ja" — Japanese (JP / Japanese / Jpan)
//   - "ko" — Korean (KR / Korean / Kore)
//
// Other languages and Latin TTCs (Helvetica.ttc, Courier.ttc) silently
// fall back to face 0 because their face names encode weight/style
// tokens rather than regional ones; callers that need a specific
// weight should use the existing weight/style API on [sfntFace], not
// language matching.
//
// Errors returned by this function wrap one of the sentinel errors
// [ErrUnknownFormat], [ErrTruncated], or [ErrCorruptTable] so callers
// can match failure modes with errors.Is.
func ParseFontForLanguage(data []byte, lang string) (Face, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("font data too short to determine format: %w", ErrTruncated)
	}
	sig := binary.BigEndian.Uint32(data[0:4])
	switch sig {
	case woffMagic:
		ttfData, err := decodeWOFF(data)
		if err != nil {
			return nil, fmt.Errorf("decode WOFF: %w", err)
		}
		return ParseTTF(ttfData)
	case ttcMagic:
		idx := max(pickFaceForLanguage(data, lang), 0)
		ttfData, err := extractTTCFont(data, idx)
		if err != nil {
			return nil, fmt.Errorf("decode TTC: %w", err)
		}
		return ParseTTF(ttfData)
	// When adding a magic to this switch, also extend
	// TestParseFontDispatchSurface in issue230_test.go so the audit
	// keeps matching what the dispatch claims to support.
	case 0x00010000, // TrueType
		0x4F54544F, // "OTTO" (OpenType/CFF)
		0x74727565: // "true" (legacy Apple TrueType)
		return ParseTTF(data)
	}
	return nil, fmt.Errorf("unknown font magic 0x%08X: %w", sig, ErrUnknownFormat)
}

// LoadFont reads and parses a font file from disk, auto-detecting the format.
// Supports TTF, OTF, and WOFF1 fonts.
//
// Errors returned by this function wrap one of the sentinel errors
// [ErrUnknownFormat], [ErrTruncated], or [ErrCorruptTable] so callers
// can match failure modes with errors.Is.
func LoadFont(path string) (Face, error) {
	return LoadFontForLanguage(path, "")
}

// LoadFontForLanguage reads and parses a font file from disk like
// [LoadFont] but, for TrueType Collections, selects the face whose
// name-table FontFamily best matches the supplied BCP-47 language tag.
// Empty lang behaves identically to [LoadFont] — face 0 for TTCs, the
// single face for standalone TTF/OTF/WOFF.
//
// Useful for paths the caller knows ahead of time will resolve to a
// pan-CJK TTC (NotoSansCJK-Regular.ttc, msyh.ttc, simsun.ttc) when the
// document context specifies a target language. See
// [ParseFontForLanguage] for the BCP-47 tag matching rules.
func LoadFontForLanguage(path, lang string) (Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read font file: %w", err)
	}
	return ParseFontForLanguage(data, lang)
}
