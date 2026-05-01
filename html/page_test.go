// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"
)

func TestPageSizeA4(t *testing.T) {
	result, err := ConvertFull(`<html><head><style>@page { size: a4; }</style></head><body><p>Text</p></body></html>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig from @page rule")
	}
	// A4: 595.28 x 841.89
	if math.Abs(result.PageConfig.Width-595.28) > 1 {
		t.Errorf("width = %.2f, want ~595.28", result.PageConfig.Width)
	}
	if math.Abs(result.PageConfig.Height-841.89) > 1 {
		t.Errorf("height = %.2f, want ~841.89", result.PageConfig.Height)
	}
}

func TestPageSizeLetter(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: letter; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if result.PageConfig.Width != 612 || result.PageConfig.Height != 792 {
		t.Errorf("size = %.0fx%.0f, want 612x792", result.PageConfig.Width, result.PageConfig.Height)
	}
}

func TestPageSizeLandscape(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: a4 landscape; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if !result.PageConfig.Landscape {
		t.Error("expected landscape flag")
	}
	// Landscape A4: width > height
	if result.PageConfig.Width <= result.PageConfig.Height {
		t.Errorf("landscape should have width > height, got %.0f x %.0f",
			result.PageConfig.Width, result.PageConfig.Height)
	}
}

func TestPageSizeCustomDimensions(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: 8.5in 11in; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	// 8.5in = 612pt, 11in = 792pt
	if math.Abs(result.PageConfig.Width-612) > 1 {
		t.Errorf("width = %.2f, want 612", result.PageConfig.Width)
	}
	if math.Abs(result.PageConfig.Height-792) > 1 {
		t.Errorf("height = %.2f, want 792", result.PageConfig.Height)
	}
}

func TestPageSizeMillimeters(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: 210mm 297mm; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	// 210mm ≈ 595.28pt, 297mm ≈ 841.89pt (A4)
	if math.Abs(result.PageConfig.Width-595.28) > 1 {
		t.Errorf("width = %.2f, want ~595.28", result.PageConfig.Width)
	}
}

func TestPageMargins(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { margin: 1in; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	for _, m := range []float64{result.PageConfig.MarginTop, result.PageConfig.MarginRight, result.PageConfig.MarginBottom, result.PageConfig.MarginLeft} {
		if math.Abs(m-72) > 1 {
			t.Errorf("margin = %.2f, want 72 (1in)", m)
		}
	}
}

func TestPageMarginsIndividual(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { margin-top: 2cm; margin-right: 1cm; margin-bottom: 2cm; margin-left: 1cm; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	// 2cm ≈ 56.69pt, 1cm ≈ 28.35pt
	if math.Abs(result.PageConfig.MarginTop-56.69) > 1 {
		t.Errorf("margin-top = %.2f, want ~56.69", result.PageConfig.MarginTop)
	}
	if math.Abs(result.PageConfig.MarginRight-28.35) > 1 {
		t.Errorf("margin-right = %.2f, want ~28.35", result.PageConfig.MarginRight)
	}
}

func TestPageSizeAndMargins(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: a4; margin: 72pt; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if math.Abs(result.PageConfig.Width-595.28) > 1 {
		t.Errorf("width = %.2f, want ~595.28", result.PageConfig.Width)
	}
	if result.PageConfig.MarginTop != 72 {
		t.Errorf("margin-top = %.2f, want 72", result.PageConfig.MarginTop)
	}
}

func TestPageSizeAutoHeight(t *testing.T) {
	// @page { size: 80mm 0; } should set width and AutoHeight=true.
	result, err := ConvertFull(`<html><head><style>@page { size: 80mm 0; margin: 0; }</style></head><body><p>Receipt</p></body></html>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig from @page rule")
	}
	// 80mm ≈ 226.77pt
	if math.Abs(result.PageConfig.Width-226.77) > 1 {
		t.Errorf("width = %.2f, want ~226.77", result.PageConfig.Width)
	}
	if result.PageConfig.Height != 0 {
		t.Errorf("height = %.2f, want 0 (auto-height)", result.PageConfig.Height)
	}
	if !result.PageConfig.AutoHeight {
		t.Error("expected AutoHeight=true for size: 80mm 0")
	}
}

func TestPageSizeAutoHeight210mm(t *testing.T) {
	// @page { size: 210mm 0; } — flyer-style auto-height.
	result, err := ConvertFull(`<html><head><style>@page { size: 210mm 0; margin: 0; }</style></head><body><h1>Hello</h1></body></html>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if math.Abs(result.PageConfig.Width-595.28) > 1 {
		t.Errorf("width = %.2f, want ~595.28", result.PageConfig.Width)
	}
	if !result.PageConfig.AutoHeight {
		t.Error("expected AutoHeight=true")
	}
}

func TestNoPageRule(t *testing.T) {
	result, _ := ConvertFull(`<p>No page rule</p>`, nil)
	if result.PageConfig != nil {
		t.Error("expected nil PageConfig when no @page rule")
	}
}

func TestBreakBeforeModernSyntax(t *testing.T) {
	elems, _ := Convert(`<div style="break-before: page">After break</div>`, nil)
	// Should have an AreaBreak before the div content.
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements (AreaBreak + content), got %d", len(elems))
	}
}

func TestBreakAfterModernSyntax(t *testing.T) {
	elems, _ := Convert(`<div style="break-after: page">Before break</div><p>After</p>`, nil)
	// Should have content + AreaBreak + content.
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}
}

func TestOrphansWidowsCSS(t *testing.T) {
	// Orphans/widows are parsed and applied to paragraphs.
	// We can't easily test the visual effect, but verify parsing doesn't error.
	elems, err := Convert(`<p style="orphans: 3; widows: 2">Text content here.</p>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Error("expected elements")
	}
}

func TestBreakInsideAvoid(t *testing.T) {
	elems, err := Convert(`<div style="break-inside: avoid"><p>Keep together</p></div>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Error("expected elements")
	}
}

// TestPageSizeWithCalc is a regression test for two connected bugs in
// the @page size parser:
//
//  1. Tokenization: `parsePageSize` used strings.Fields, which split
//     functional values like calc(8in + 0.5in) on internal whitespace.
//     Pre-fix `@page { size: calc(8in + 0.5in) 11in }` became 4 tokens
//     ["calc(8in", "+", "0.5in)", "11in"], parts[0] = "calc(8in"
//     failed parseCSSLength → w stayed 0, so the `w > 0` guard
//     suppressed any size assignment and the page silently kept the
//     default. Same root cause as #236, #237, #240, #242, #244.
//  2. The page-local `parseCSSLength` only stripped known unit suffixes
//     (in/mm/cm/pt/px) and did not recognize calc()/min()/max()/clamp()
//     at all. So even after fixing tokenization, the bare calc token
//     would still parse to 0. Switched to `parseLengthPt`, which
//     routes through the main `parseLength` (calc-aware) and avoids
//     the page-local `parseSingleLength`/`parseCSSLengthWithUnit`.
//
// fontSize is now plumbed through parsePageSize so em/rem inside calc
// resolve correctly. parsePageConfig already had defaultFontSize.
func TestPageSizeWithCalc(t *testing.T) {
	tests := []struct {
		name           string
		size           string
		wantWidth      float64
		wantHeight     float64
		wantLandscape  bool
		wantAutoHeight bool
	}{
		{
			name: "calc width, plain in height",
			size: "calc(8in + 0.5in) 11in",
			// 8in + 0.5in = 8.5in = 612pt; 11in = 792pt.
			wantWidth: 612, wantHeight: 792,
		},
		{
			name:      "calc on both axes",
			size:      "calc(8in + 0.5in) calc(10in + 1in)",
			wantWidth: 612, wantHeight: 792,
		},
		{
			name: "calc with subtraction",
			size: "calc(9in - 0.5in) 11in",
			// 8.5in = 612pt; 11in = 792pt.
			wantWidth: 612, wantHeight: 792,
		},
		{
			name: "calc with multiplication",
			size: "calc(2in * 4) 11in",
			// 8in = 576pt; 11in = 792pt.
			wantWidth: 576, wantHeight: 792,
		},
		{
			name: "calc with division",
			size: "calc(17in / 2) 11in",
			// 8.5in = 612pt; 11in = 792pt.
			wantWidth: 612, wantHeight: 792,
		},
		{
			name: "calc with mm units",
			size: "calc(105mm + 105mm) 297mm",
			// 210mm = 595.28pt; 297mm = 841.89pt (A4).
			wantWidth: 595.28, wantHeight: 841.89,
		},
		{
			name: "calc with cm units",
			size: "calc(10cm + 11cm) 29.7cm",
			// 21cm = 595.28pt; 29.7cm = 841.89pt.
			wantWidth: 595.28, wantHeight: 841.89,
		},
		{
			name: "mixed units: calc(in) and mm",
			size: "calc(8in + 0.5in) 297mm",
			// 8.5in = 612pt; 297mm = 841.89pt.
			wantWidth: 612, wantHeight: 841.89,
		},
		{
			name: "min() width, max() height",
			size: "min(8.5in, 9in) max(10in, 11in)",
			// min picks 8.5in = 612pt; max picks 11in = 792pt.
			wantWidth: 612, wantHeight: 792,
		},
		{
			name: "clamp() width",
			size: "clamp(7in, 8.5in, 10in) 11in",
			// clamp middle = 8.5in = 612pt.
			wantWidth: 612, wantHeight: 792,
		},
		{
			name: "calc with landscape orientation",
			size: "calc(8in + 0.5in) 11in landscape",
			// landscape swaps: width becomes 792pt, height 612pt.
			wantWidth: 792, wantHeight: 612, wantLandscape: true,
		},
		{
			name: "single calc value → square page",
			size: "calc(5in + 1in)",
			// 6in = 432pt, applied to both axes.
			wantWidth: 432, wantHeight: 432,
		},
		{
			name:      "tab and newline as separators",
			size:      "calc(8in + 0.5in)\t11in",
			wantWidth: 612, wantHeight: 792,
		},
		{
			name: "calc width + zero height (auto-height special case)",
			size: "calc(8in + 0.5in) 0",
			// 8.5in = 612pt; literal "0" triggers AutoHeight.
			wantWidth: 612, wantHeight: 0, wantAutoHeight: true,
		},
		{
			name: "unbalanced calc paren: parser silently drops the size",
			// splitTopLevelFields keeps the unbalanced calc + trailing
			// characters as one token (depth never returns to 0). The
			// `w > 0` guard then suppresses any size assignment, so width
			// and height stay at zero. Documents the no-crash invariant.
			size:      "calc(8in + 0.5in 11in",
			wantWidth: 0, wantHeight: 0,
		},
		{
			name: "3+ tokens: extras silently ignored",
			// CSS @page size accepts 1 or 2 dimensions plus an
			// optional orientation. Anything beyond is ignored; the
			// first two tokens become width/height.
			size:      "8.5in 11in foo bar",
			wantWidth: 612, wantHeight: 792,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			htmlStr := `<html><head><style>@page { size: ` + tt.size + `; }</style></head><body><p>X</p></body></html>`
			result, err := ConvertFull(htmlStr, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.PageConfig == nil {
				t.Fatal("expected PageConfig from @page rule")
			}
			if math.Abs(result.PageConfig.Width-tt.wantWidth) > 1 {
				t.Errorf("width = %.2f, want ~%.2f", result.PageConfig.Width, tt.wantWidth)
			}
			if math.Abs(result.PageConfig.Height-tt.wantHeight) > 1 {
				t.Errorf("height = %.2f, want ~%.2f", result.PageConfig.Height, tt.wantHeight)
			}
			if result.PageConfig.Landscape != tt.wantLandscape {
				t.Errorf("landscape = %v, want %v", result.PageConfig.Landscape, tt.wantLandscape)
			}
			if result.PageConfig.AutoHeight != tt.wantAutoHeight {
				t.Errorf("AutoHeight = %v, want %v",
					result.PageConfig.AutoHeight, tt.wantAutoHeight)
			}
		})
	}
}
