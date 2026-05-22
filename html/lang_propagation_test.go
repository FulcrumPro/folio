// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"path/filepath"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// TestFindHTMLLangExtractsAttribute covers the helper that walks the
// parsed document tree looking for the root <html> element's `lang`.
// Mirrors the BCP-47 tags the @font-face loader will route to
// font.ParseFontForLanguage; absent and malformed cases must yield
// an empty string so the upstream call falls back to face-0.
func TestFindHTMLLangExtractsAttribute(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"zh-CN", `<html lang="zh-CN"><body>x</body></html>`, "zh-CN"},
		{"ja", `<html lang="ja"><body>x</body></html>`, "ja"},
		{"en-US", `<html lang="en-US"><body>x</body></html>`, "en-US"},
		{"no attribute", `<html><body>x</body></html>`, ""},
		{"empty value", `<html lang=""><body>x</body></html>`, ""},
		{"doctype before html", `<!DOCTYPE html><html lang="ko"><body>x</body></html>`, "ko"},
		{"no html element (fragment)", `<body>x</body>`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := xhtml.Parse(strings.NewReader(tc.in))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := findHTMLLang(doc); got != tc.want {
				t.Errorf("findHTMLLang = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestConvertFullPopulatesMetadataLanguage verifies that the document
// lang reaches ConvertResult.Metadata.Language — the callable surface
// for downstream code that needs the same hint (PDF /Lang catalog
// entry, future per-element lookups, etc.).
func TestConvertFullPopulatesMetadataLanguage(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "zh-CN reaches metadata",
			html: `<!DOCTYPE html><html lang="zh-CN"><body><p>测试</p></body></html>`,
			want: "zh-CN",
		},
		{
			name: "absent lang yields empty metadata",
			html: `<!DOCTYPE html><html><body><p>x</p></body></html>`,
			want: "",
		},
		{
			name: "ja reaches metadata",
			html: `<!DOCTYPE html><html lang="ja"><body><p>テスト</p></body></html>`,
			want: "ja",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ConvertFull(tc.html, nil)
			if err != nil {
				t.Fatalf("ConvertFull: %v", err)
			}
			if result.Metadata.Language != tc.want {
				t.Errorf("Metadata.Language = %q, want %q", result.Metadata.Language, tc.want)
			}
		})
	}
}

// TestLangPropagationDoesNotBreakFontFaceLoad runs the full @font-face
// pipeline with both a langless document and a lang-tagged one,
// against the same single-face TTF fixture. Because the fixture has
// only one face, ParseFontForLanguage and ParseFont return
// identical results — but the test pins that the lang propagation
// path didn't break @font-face loading for the common case.
//
// End-to-end SC-vs-JP face selection from a real multi-face TTC is
// exercised by font/ttc_lang_test.go::TestParseFontForLanguage*
// against the same parser this PR's wiring routes through. Pinning
// the html-side wiring there would require duplicating the multi-
// face TTC builder into this package; for the @font-face wiring the
// invariant that matters is "loading still works when lang is set"
// and "Metadata.Language is observable to callers."
func TestLangPropagationDoesNotBreakFontFaceLoad(t *testing.T) {
	abs, err := filepath.Abs("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	for _, tc := range []struct {
		name string
		lang string
	}{
		{"no lang (back-compat)", ""},
		{"with zh-CN lang", "zh-CN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			html := buildSyntheticCJKHTML(abs, tc.lang)
			result, err := ConvertFull(html, &Options{StrictAssets: true})
			if err != nil {
				t.Fatalf("ConvertFull: %v", err)
			}
			if result.Metadata.Language != tc.lang {
				t.Errorf("Metadata.Language = %q, want %q", result.Metadata.Language, tc.lang)
			}
			// At least one element rendered — the @font-face load
			// did not abort the conversion.
			if len(result.Elements) == 0 {
				t.Error("ConvertFull produced no elements; @font-face load may have failed")
			}
		})
	}
}

// buildSyntheticCJKHTML constructs a minimal HTML document that
// loads the synthetic CJK TTF via @font-face url() and renders one
// CJK character with it. The lang argument is omitted when empty,
// matching how a real <html> element without a lang attribute is
// emitted.
func buildSyntheticCJKHTML(ttfPath, lang string) string {
	openTag := "<html>"
	if lang != "" {
		openTag = `<html lang="` + lang + `">`
	}
	return openTag + `<head><style>
@font-face { font-family: 'Synth'; src: url('` + ttfPath + `'); }
body { font-family: 'Synth'; }
</style></head><body><p>中</p></body></html>`
}
