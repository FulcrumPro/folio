// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestAddHTMLMetadata(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	err := doc.AddHTML(`<!DOCTYPE html>
<html>
<head>
  <title>Quarterly Report</title>
  <meta name="author" content="Jane Doe">
  <meta name="subject" content="Finance">
  <meta name="keywords" content="quarterly, revenue, 2026">
  <meta name="generator" content="ReportBuilder v2">
</head>
<body>
  <h1>Q1 2026 Revenue</h1>
  <p>Revenue grew 23% year over year.</p>
</body>
</html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}

	if doc.Info.Title != "Quarterly Report" {
		t.Errorf("Title = %q, want %q", doc.Info.Title, "Quarterly Report")
	}
	if doc.Info.Author != "Jane Doe" {
		t.Errorf("Author = %q, want %q", doc.Info.Author, "Jane Doe")
	}
	if doc.Info.Subject != "Finance" {
		t.Errorf("Subject = %q, want %q", doc.Info.Subject, "Finance")
	}
	if doc.Info.Keywords != "quarterly, revenue, 2026" {
		t.Errorf("Keywords = %q, want %q", doc.Info.Keywords, "quarterly, revenue, 2026")
	}
	if doc.Info.Creator != "ReportBuilder v2" {
		t.Errorf("Creator = %q, want %q", doc.Info.Creator, "ReportBuilder v2")
	}

	if len(doc.elements) == 0 {
		t.Error("expected layout elements from HTML body")
	}
}

func TestAddHTMLPreservesExistingInfo(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "My Custom Title"

	err := doc.AddHTML(`<html><head><title>HTML Title</title></head><body><p>Hi</p></body></html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}

	if doc.Info.Title != "My Custom Title" {
		t.Errorf("Title was overwritten: got %q, want %q", doc.Info.Title, "My Custom Title")
	}
}

func TestAddHTMLProducesPDF(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	err := doc.AddHTML(`<html>
<head><title>Test PDF</title><meta name="author" content="Folio"></head>
<body>
  <h1>Hello World</h1>
  <p>This PDF was generated from HTML with automatic metadata.</p>
</body>
</html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}

	var buf bytes.Buffer
	n, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if n == 0 {
		t.Fatal("WriteTo produced zero bytes")
	}

	pdf := buf.String()
	if !strings.HasPrefix(pdf, "%PDF-") {
		t.Error("output does not start with %PDF-")
	}
	if !strings.Contains(pdf, "Test PDF") {
		t.Error("PDF does not contain title metadata")
	}
}

// TestAddHTMLFirstMarginsMergeOverBase is the Defect B end-to-end regression
// through the document entry point: a partial @page :first override inherits the
// base @page {} margins for sides it does not set.
func TestAddHTMLFirstMarginsMergeOverBase(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	err := doc.AddHTML(`<html><head><style>
		@page { margin: 2cm; }
		@page :first { margin-top: 4cm; }
	</style></head><body><p>X</p></body></html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	if doc.firstMargins == nil {
		t.Fatal("expected firstMargins to be set")
	}
	// 4cm ≈ 113.39pt, 2cm ≈ 56.69pt.
	const cm2, cm4 = 56.69, 113.39
	if d := doc.firstMargins.Top - cm4; d > 1 || d < -1 {
		t.Errorf("firstMargins.Top = %.2f, want ~113.39 (4cm)", doc.firstMargins.Top)
	}
	if d := doc.firstMargins.Right - cm2; d > 1 || d < -1 {
		t.Errorf("firstMargins.Right = %.2f, want ~56.69 (2cm inherited, not 0)", doc.firstMargins.Right)
	}
	if d := doc.firstMargins.Bottom - cm2; d > 1 || d < -1 {
		t.Errorf("firstMargins.Bottom = %.2f, want ~56.69 (2cm inherited, not 0)", doc.firstMargins.Bottom)
	}
	if d := doc.firstMargins.Left - cm2; d > 1 || d < -1 {
		t.Errorf("firstMargins.Left = %.2f, want ~56.69 (2cm inherited, not 0)", doc.firstMargins.Left)
	}
}

// TestAddHTMLLeftRightMarginsAndBoxes verifies :left/:right margins and margin
// boxes are plumbed from HTML through the document to the renderer, and that the
// resulting multi-page PDF renders without error.
func TestAddHTMLLeftRightMarginsAndBoxes(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	err := doc.AddHTML(`<html><head><style>
		@page { margin: 2cm; }
		@page :left { margin-left: 3cm; @top-left { content: "L"; } }
		@page :right { margin-right: 3cm; @top-right { content: "R"; } }
	</style></head><body>
		<p style="page-break-after: always;">one</p>
		<p style="page-break-after: always;">two</p>
		<p>three</p>
	</body></html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	if doc.leftMargins == nil || doc.rightMargins == nil {
		t.Fatal("expected left and right margins to be set")
	}
	if doc.leftMarginBoxes == nil || doc.rightMarginBoxes == nil {
		t.Fatal("expected left and right margin boxes to be set")
	}
	// 3cm ≈ 85.04pt.
	if d := doc.leftMargins.Left - 85.04; d > 1 || d < -1 {
		t.Errorf("leftMargins.Left = %.2f, want ~85.04 (3cm)", doc.leftMargins.Left)
	}
	if d := doc.rightMargins.Right - 85.04; d > 1 || d < -1 {
		t.Errorf("rightMargins.Right = %.2f, want ~85.04 (3cm)", doc.rightMargins.Right)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "%PDF-") {
		t.Error("output does not start with %PDF-")
	}
}

func TestAddHTMLTemplate(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	type Item struct {
		Name  string
		Price string
	}
	data := struct {
		Title string
		Items []Item
	}{
		Title: "Invoice #42",
		Items: []Item{
			{"Widget", "$10.00"},
			{"Gadget", "$25.00"},
		},
	}

	err := doc.AddHTMLTemplate(`
		<h1>{{.Title}}</h1>
		<table>
		{{range .Items}}
			<tr><td>{{.Name}}</td><td>{{.Price}}</td></tr>
		{{end}}
		</table>
	`, data, nil)
	if err != nil {
		t.Fatalf("AddHTMLTemplate: %v", err)
	}

	var buf bytes.Buffer
	n, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if n == 0 {
		t.Fatal("produced zero bytes")
	}
	// Content is in compressed streams, so just verify non-trivial size.
	if buf.Len() < 500 {
		t.Errorf("PDF seems too small for template with table: %d bytes", buf.Len())
	}
}

func TestAddHTMLTemplateInvalidTemplate(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	err := doc.AddHTMLTemplate(`{{.Unclosed`, nil, nil)
	if err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestAddHTMLTemplateExecutionError(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	// Template calls a method on nil — should fail at execute time.
	err := doc.AddHTMLTemplate(`<p>{{call .Fn}}</p>`, struct{ Fn func() string }{nil}, nil)
	if err == nil {
		t.Error("expected error for template execution failure")
	}
}

func TestAddHTMLTemplateFuncs(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	funcs := template.FuncMap{
		"upper": strings.ToUpper,
	}

	err := doc.AddHTMLTemplateFuncs(`<p>{{upper .Name}}</p>`, funcs, struct{ Name string }{"hello"}, nil)
	if err != nil {
		t.Fatalf("AddHTMLTemplateFuncs: %v", err)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	// Just verify it produces a valid PDF — content is compressed.
	if !strings.HasPrefix(buf.String(), "%PDF-") {
		t.Error("output does not start with %PDF-")
	}
}

func TestAddHTMLTemplateWithCSS(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	err := doc.AddHTMLTemplate(`
		<style>
			h1 { color: red; font-size: 24px; }
			.item { font-weight: bold; }
		</style>
		<h1>{{.Title}}</h1>
		<p class="item">{{.Body}}</p>
	`, struct{ Title, Body string }{"Report", "Content here"}, nil)
	if err != nil {
		t.Fatalf("AddHTMLTemplate with CSS: %v", err)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("produced empty PDF")
	}
}

func nearly(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.5
}

// B2: @page margin percentages must resolve against the page box —
// left/right against page width, top/bottom against page height.
func TestAddHTMLPageMarginPercent(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	err := doc.AddHTML(`<html><head><style>
		@page { size: A4; margin: 10%; }
	</style></head><body><p>X</p></body></html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	// A4 = 595.28 x 841.89; 10% of each dimension.
	wantLR := 0.10 * 595.28
	wantTB := 0.10 * 841.89
	if !nearly(doc.margins.Left, wantLR) || !nearly(doc.margins.Right, wantLR) {
		t.Errorf("L/R margins = %.2f/%.2f, want ~%.2f (10%% of width)",
			doc.margins.Left, doc.margins.Right, wantLR)
	}
	if !nearly(doc.margins.Top, wantTB) || !nearly(doc.margins.Bottom, wantTB) {
		t.Errorf("T/B margins = %.2f/%.2f, want ~%.2f (10%% of height)",
			doc.margins.Top, doc.margins.Bottom, wantTB)
	}
}

// B2: percentage longhands resolve against the matching page dimension.
func TestAddHTMLPageMarginPercentLonghand(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	err := doc.AddHTML(`<html><head><style>
		@page { size: A4; margin-top: 5%; margin-left: 20%; }
	</style></head><body><p>X</p></body></html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	if want := 0.05 * 841.89; !nearly(doc.margins.Top, want) {
		t.Errorf("margin-top = %.2f, want ~%.2f (5%% of height)", doc.margins.Top, want)
	}
	if want := 0.20 * 595.28; !nearly(doc.margins.Left, want) {
		t.Errorf("margin-left = %.2f, want ~%.2f (20%% of width)", doc.margins.Left, want)
	}
}

// N1: @page margin longhands must be calc-aware. Before the fix
// margin-top: calc(1in + 2px) resolved to 0 (non-calc parser).
func TestAddHTMLPageMarginCalcLonghand(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	err := doc.AddHTML(`<html><head><style>
		@page { size: A4; margin-top: calc(1in + 2px); }
	</style></head><body><p>X</p></body></html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	// 1in = 72pt, 2px = 1.5pt → 73.5pt.
	if want := 72.0 + 1.5; !nearly(doc.margins.Top, want) {
		t.Errorf("margin-top = %.2f, want ~%.2f (calc 1in + 2px)", doc.margins.Top, want)
	}
}

// S3: @page { size: landscape } with no explicit size must rotate the
// document default page size (Letter 612x792 → 792x612).
func TestAddHTMLPageOrientationOnlyLandscape(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	err := doc.AddHTML(`<html><head><style>
		@page { size: landscape; }
	</style></head><body><p>X</p></body></html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	if !nearly(doc.pageSize.Width, 792) || !nearly(doc.pageSize.Height, 612) {
		t.Errorf("page size = %.0fx%.0f, want 792x612 (landscape Letter)",
			doc.pageSize.Width, doc.pageSize.Height)
	}
}

// S3: portrait keyword on an already-portrait default is a no-op, and a
// named-size + orientation keyword (size: a4 landscape) keeps working.
func TestAddHTMLPageOrientationOnlyPortraitNoop(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	if err := doc.AddHTML(`<html><head><style>@page { size: portrait; }</style></head><body><p>X</p></body></html>`, nil); err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	if !nearly(doc.pageSize.Width, 612) || !nearly(doc.pageSize.Height, 792) {
		t.Errorf("portrait page size = %.0fx%.0f, want 612x792", doc.pageSize.Width, doc.pageSize.Height)
	}

	doc2 := NewDocument(PageSizeLetter)
	if err := doc2.AddHTML(`<html><head><style>@page { size: a4 landscape; }</style></head><body><p>X</p></body></html>`, nil); err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	// A4 landscape: 841.89 x 595.28 (width > height).
	if doc2.pageSize.Width <= doc2.pageSize.Height {
		t.Errorf("a4 landscape = %.0fx%.0f, want width > height", doc2.pageSize.Width, doc2.pageSize.Height)
	}
}
