// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

// TestBodyCounterInline verifies that CSS counter(page) and counter(pages)
// inside body-flow content (via ::before pseudo-element) resolve to the
// correct values per page. This is the primary acceptance test for the
// body-flow counter feature.
func TestBodyCounterInline(t *testing.T) {
	htmlSrc := `
<html>
<head><style>
p.page-of::before { content: "Page " counter(page) " of " counter(pages); }
</style></head>
<body>
<p class="page-of"></p>
<p>` + strings.Repeat("Lorem ipsum dolor sit amet. ", 200) + `</p>
<p class="page-of"></p>
<p>` + strings.Repeat("Consectetur adipiscing elit. ", 200) + `</p>
<p class="page-of"></p>
</body>
</html>`

	doc := document.NewDocument(document.PageSizeLetter)
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	total := r.PageCount()
	if total < 2 {
		t.Fatalf("expected multi-page document, got %d page(s)", total)
	}

	// First body-flow counter sits on page 1 → "Page 1 of N".
	p1, _ := r.Page(0)
	t1, _ := p1.ExtractText()
	want1 := fmt.Sprintf("Page 1 of %d", total)
	if !strings.Contains(t1, want1) {
		t.Errorf("page 1 should contain %q, got: %s", want1, truncate(t1, 300))
	}

	// Last body-flow counter sits on the last page → "Page N of N".
	last := total - 1
	pN, _ := r.Page(last)
	tN, _ := pN.ExtractText()
	wantN := fmt.Sprintf("Page %d of %d", last+1, total)
	if !strings.Contains(tN, wantN) {
		t.Errorf("page %d should contain %q, got: %s", last+1, wantN, truncate(tN, 300))
	}
}

// TestBodyCounterAgreesWithMarginBox verifies that body-flow counter(page)
// resolves to the same value as the @page margin-box counter(page) on the
// same page — the two render sites share one counter machinery per the
// design.
func TestBodyCounterAgreesWithMarginBox(t *testing.T) {
	htmlSrc := `
<html>
<head><style>
@page { @bottom-center { content: "footer page " counter(page); } }
p.body-page::before { content: "body page " counter(page); }
</style></head>
<body>
<p class="body-page"></p>
<p>` + strings.Repeat("Filler content. ", 300) + `</p>
<p class="body-page"></p>
</body>
</html>`

	doc := document.NewDocument(document.PageSizeLetter)
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if r.PageCount() < 2 {
		t.Fatalf("expected at least 2 pages, got %d", r.PageCount())
	}

	for i := 0; i < r.PageCount(); i++ {
		p, _ := r.Page(i)
		text, _ := p.ExtractText()
		// Footer is always present on each page.
		wantFooter := fmt.Sprintf("footer page %d", i+1)
		if !strings.Contains(text, wantFooter) {
			t.Errorf("page %d footer should contain %q, got: %s", i+1, wantFooter, truncate(text, 300))
			continue
		}
		// Body counter only appears on pages that contain a .body-page span;
		// when present it must match the footer value (same machinery).
		wantBody := fmt.Sprintf("body page %d", i+1)
		if strings.Contains(text, "body page") && !strings.Contains(text, wantBody) {
			t.Errorf("page %d body counter mismatch: want %q, got: %s", i+1, wantBody, truncate(text, 300))
		}
	}
}

// TestBodyCounterPagesPlaceholderNotLeaked guards against the legacy
// ##TOTAL_PAGES## placeholder leaking into rendered output. The two-pass
// emission resolves counter(pages) directly during draw — the placeholder
// must never reach the content stream.
func TestBodyCounterPagesPlaceholderNotLeaked(t *testing.T) {
	htmlSrc := `
<html>
<head><style>
@page { @bottom-center { content: counter(pages); } }
p.tot::before { content: counter(pages); }
</style></head>
<body>
<p class="tot"></p>
<p>` + strings.Repeat("Body. ", 200) + `</p>
</body>
</html>`

	doc := document.NewDocument(document.PageSizeLetter)
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("##TOTAL_PAGES##")) {
		t.Error("##TOTAL_PAGES## placeholder leaked into PDF output")
	}
}

// TestBodyCounterStableAcrossDigitTransition documents that line layout
// is decided once at measurement time, with width reserved for a fixed
// digit budget. The number of counter-bearing blocks that fit on page 1
// must not shift as digit count grows from 1 to 2 (page 9 → 10) — width
// is reserved at measurement time for a max-digit string, so pagination
// is independent of the resolved digit count.
func TestBodyCounterStableAcrossDigitTransition(t *testing.T) {
	mkDoc := func(forcedPages int) *bytes.Buffer {
		var body strings.Builder
		// Many counter-bearing paragraphs at the top — each renders
		// a single counter via ::before. Their height (and therefore
		// pagination) must depend only on reserved width, not on the
		// digit count of the resolved value.
		for i := 0; i < 40; i++ {
			body.WriteString(`<p class="cnt"></p>`)
		}
		// Force the chosen page count via page-break-after blocks.
		for i := 0; i < forcedPages-1; i++ {
			body.WriteString(`<p style="page-break-after:always">x</p>`)
		}
		htmlSrc := `<html><head><style>
p.cnt::before { content: counter(page) "/" counter(pages); }
</style></head><body>` + body.String() + `</body></html>`

		doc := document.NewDocument(document.PageSizeLetter)
		if err := doc.AddHTML(htmlSrc, nil); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := doc.WriteTo(&buf); err != nil {
			t.Fatal(err)
		}
		return &buf
	}

	count := func(s, sub string) int {
		n := 0
		for i := 0; ; {
			j := strings.Index(s[i:], sub)
			if j < 0 {
				return n
			}
			n++
			i += j + len(sub)
		}
	}

	// 9 pages → counter is "1/9" (3 chars). 12 pages → "1/12" (4 chars).
	out9 := mkDoc(9)
	r9, err := reader.Parse(out9.Bytes())
	if err != nil {
		t.Fatalf("n=9: parse failed: %v", err)
	}
	p9, _ := r9.Page(0)
	t9, _ := p9.ExtractText()

	out12 := mkDoc(12)
	r12, err := reader.Parse(out12.Bytes())
	if err != nil {
		t.Fatalf("n=12: parse failed: %v", err)
	}
	p12, _ := r12.Page(0)
	t12, _ := p12.ExtractText()

	// Both totals must resolve correctly.
	if !strings.Contains(t9, fmt.Sprintf("1/%d", r9.PageCount())) {
		t.Errorf("9-page doc: expected counter %q in page 1, got: %s",
			fmt.Sprintf("1/%d", r9.PageCount()), truncate(t9, 300))
	}
	if !strings.Contains(t12, fmt.Sprintf("1/%d", r12.PageCount())) {
		t.Errorf("12-page doc: expected counter %q in page 1, got: %s",
			fmt.Sprintf("1/%d", r12.PageCount()), truncate(t12, 300))
	}

	// Geometry stability: the number of counter-bearing paragraphs that
	// land on page 1 must be the same in both documents. If width
	// reservation drifted with digit count, fewer would fit when the
	// resolved counter is 4 chars vs 3 chars.
	c9 := count(t9, fmt.Sprintf("1/%d", r9.PageCount()))
	c12 := count(t12, fmt.Sprintf("1/%d", r12.PageCount()))
	if c9 != c12 {
		t.Errorf("digit-transition layout instability: page 1 has %d counter instances at 9 pages but %d at 12 pages", c9, c12)
	}
	if c9 == 0 {
		t.Errorf("expected counter instances on page 1, got none")
	}
}

// TestBodyCounterUnusedZeroOverhead is a smoke test for the
// no-regression promise: documents that don't use counter() in body
// flow must produce byte-identical output across runs and contain no
// leftover placeholder syntax.
func TestBodyCounterUnusedZeroOverhead(t *testing.T) {
	htmlSrc := `
<html><body>
<p>Plain content with no counter references.</p>
<p>` + strings.Repeat("Word. ", 50) + `</p>
</body></html>`

	render := func() []byte {
		doc := document.NewDocument(document.PageSizeLetter)
		if err := doc.AddHTML(htmlSrc, nil); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := doc.WriteTo(&buf); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}

	out1 := render()
	out2 := render()

	leftover := regexp.MustCompile(`\{counter\((page|pages)\)\}`)
	if leftover.Find(out1) != nil {
		t.Error("unexpected counter placeholder in output for document that uses no counters")
	}
	if !bytes.Equal(out1, out2) {
		t.Errorf("non-counter document not deterministic: %d vs %d bytes", len(out1), len(out2))
	}
}

// TestBodyCounterInTableCell verifies that counter(page) resolves
// inside a table cell — the body-flow counter mechanism must work for
// content nested in <td> just as it does for free-flow blocks.
func TestBodyCounterInTableCell(t *testing.T) {
	htmlSrc := `
<html>
<head><style>
p.pg::before { content: "p" counter(page); }
</style></head>
<body>
<table><tr><td><p class="pg"></p></td><td>data</td></tr></table>
<p>` + strings.Repeat("Filler. ", 200) + `</p>
<table><tr><td><p class="pg"></p></td><td>more</td></tr></table>
<p>` + strings.Repeat("More filler. ", 200) + `</p>
<table><tr><td><p class="pg"></p></td><td>last</td></tr></table>
</body>
</html>`

	doc := document.NewDocument(document.PageSizeLetter)
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	// First table cell counter sits on page 1 — must resolve to "p1".
	p1, _ := r.Page(0)
	t1, _ := p1.ExtractText()
	if !strings.Contains(t1, "p1") {
		t.Errorf("page 1 table cell should contain 'p1', got: %s", truncate(t1, 300))
	}

	// Last table cell counter must equal its page number.
	last := r.PageCount() - 1
	pN, _ := r.Page(last)
	tN, _ := pN.ExtractText()
	wantN := fmt.Sprintf("p%d", last+1)
	if !strings.Contains(tN, wantN) {
		t.Errorf("last page table cell should contain %q, got: %s", wantN, truncate(tN, 300))
	}

	// Placeholder must never leak.
	if bytes.Contains(buf.Bytes(), []byte("{counter(page)}")) {
		t.Error("counter placeholder leaked through table-cell rendering")
	}
}

// TestBodyCounterSinglePage verifies the degenerate case where the
// document fits on one page — counter(pages) must resolve to 1.
func TestBodyCounterSinglePage(t *testing.T) {
	htmlSrc := `
<html>
<head><style>
p.tot::before { content: "Page " counter(page) " of " counter(pages); }
</style></head>
<body>
<p class="tot"></p>
<p>Short content.</p>
</body>
</html>`

	doc := document.NewDocument(document.PageSizeLetter)
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if r.PageCount() != 1 {
		t.Fatalf("expected single-page document, got %d pages", r.PageCount())
	}
	p1, _ := r.Page(0)
	t1, _ := p1.ExtractText()
	if !strings.Contains(t1, "Page 1 of 1") {
		t.Errorf("single-page doc should resolve to 'Page 1 of 1', got: %s", truncate(t1, 300))
	}
}

// TestBodyCounterFirstPageMargin verifies that counter() inside a
// @page :first margin box resolves to 1 (a common cover-page pattern).
func TestBodyCounterFirstPageMargin(t *testing.T) {
	htmlSrc := `
<html>
<head><style>
@page :first { @top-center { content: "cover " counter(page) "/" counter(pages); } }
@page { @bottom-center { content: "p" counter(page) "/" counter(pages); } }
</style></head>
<body>
<p>Cover.</p>
<p style="page-break-before:always">` + strings.Repeat("Body. ", 200) + `</p>
<p style="page-break-before:always">` + strings.Repeat("More. ", 200) + `</p>
</body>
</html>`

	doc := document.NewDocument(document.PageSizeLetter)
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	total := r.PageCount()
	if total < 2 {
		t.Fatalf("expected multi-page doc, got %d", total)
	}
	p1, _ := r.Page(0)
	t1, _ := p1.ExtractText()
	wantCover := fmt.Sprintf("cover 1/%d", total)
	if !strings.Contains(t1, wantCover) {
		t.Errorf("first page :first margin should contain %q, got: %s", wantCover, truncate(t1, 300))
	}
	wantFooter := fmt.Sprintf("p1/%d", total)
	if !strings.Contains(t1, wantFooter) {
		t.Errorf("first page footer should contain %q, got: %s", wantFooter, truncate(t1, 300))
	}
}
