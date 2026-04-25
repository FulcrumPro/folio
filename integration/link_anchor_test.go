// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

// parsedLink is a single /Subtype /Link annotation with the page it
// lives on and the resolved kind/target. Built by collectLinks below.
type parsedLink struct {
	srcPage  int    // 0-based page index where the annotation appears
	kind     string // "Dest", "URI", "GoTo", or "" (unknown / malformed)
	uri      string // populated when kind == "URI"
	destStr  string // populated when kind == "GoTo" (string-based fallback)
	destPage int    // 0-based page index referenced by /Dest, -1 if unresolved
}

// collectLinks parses the PDF and returns every /Subtype /Link annotation
// across all pages, with /Dest resolved back to the destination page index
// via the catalog's page tree. This is the structural assertion that
// substring matching can't give us — we know exactly which page each link
// sits on AND which page it jumps to.
func collectLinks(t *testing.T, data []byte) []parsedLink {
	t.Helper()
	r := parsePDF(t, data)
	pageByRef := buildPageNumByRef(t, r)

	var out []parsedLink
	for i := range r.PageCount() {
		page, err := r.Page(i)
		if err != nil {
			t.Fatalf("Page(%d): %v", i, err)
		}
		annotsObj := page.Dict().Get("Annots")
		if annotsObj == nil {
			continue
		}
		resolved, err := r.ResolveObject(annotsObj)
		if err != nil {
			t.Fatalf("resolve Annots on page %d: %v", i, err)
		}
		arr, ok := resolved.(*core.PdfArray)
		if !ok {
			continue
		}
		for _, annObj := range arr.All() {
			annResolved, err := r.ResolveObject(annObj)
			if err != nil {
				continue
			}
			d, ok := annResolved.(*core.PdfDictionary)
			if !ok {
				continue
			}
			sub, _ := d.Get("Subtype").(*core.PdfName)
			if sub == nil || sub.Value != "Link" {
				continue
			}
			pl := parsedLink{srcPage: i, destPage: -1}
			if destObj := d.Get("Dest"); destObj != nil {
				pl.kind = "Dest"
				if destArr, ok := destObj.(*core.PdfArray); ok && destArr.Len() >= 1 {
					if ref, ok := destArr.At(0).(*core.PdfIndirectReference); ok {
						if pidx, ok := pageByRef[ref.Num()]; ok {
							pl.destPage = pidx
						}
					}
				}
			} else if aObj := d.Get("A"); aObj != nil {
				aResolved, err := r.ResolveObject(aObj)
				if err == nil {
					if ad, ok := aResolved.(*core.PdfDictionary); ok {
						if s, _ := ad.Get("S").(*core.PdfName); s != nil {
							pl.kind = s.Value
							if s.Value == "URI" {
								if uri, _ := ad.Get("URI").(*core.PdfString); uri != nil {
									pl.uri = uri.Text()
								}
							}
							if s.Value == "GoTo" {
								if dst, _ := ad.Get("D").(*core.PdfString); dst != nil {
									pl.destStr = dst.Text()
								}
							}
						}
					}
				}
			}
			out = append(out, pl)
		}
	}
	return out
}

// buildPageNumByRef maps page object number → 0-based page index by walking
// the catalog's flat /Pages.Kids array. Folio writes a flat page tree, so a
// single Kids walk suffices. The reader doesn't expose this map directly.
func buildPageNumByRef(t *testing.T, r *reader.PdfReader) map[int]int {
	t.Helper()
	cat := r.Catalog()
	pagesObj := cat.Get("Pages")
	if pagesObj == nil {
		t.Fatal("catalog has no /Pages")
	}
	pagesResolved, err := r.ResolveObject(pagesObj)
	if err != nil {
		t.Fatalf("resolve /Pages: %v", err)
	}
	pagesDict, ok := pagesResolved.(*core.PdfDictionary)
	if !ok {
		t.Fatal("/Pages did not resolve to a dictionary")
	}
	kidsObj := pagesDict.Get("Kids")
	if kidsObj == nil {
		t.Fatal("/Pages has no /Kids")
	}
	kidsResolved, err := r.ResolveObject(kidsObj)
	if err != nil {
		t.Fatalf("resolve /Kids: %v", err)
	}
	kidsArr, ok := kidsResolved.(*core.PdfArray)
	if !ok {
		t.Fatal("/Kids is not an array")
	}
	out := map[int]int{}
	for i, kid := range kidsArr.All() {
		ref, ok := kid.(*core.PdfIndirectReference)
		if !ok {
			t.Fatalf("/Kids[%d] is not an indirect reference (nested page tree not supported by this test harness)", i)
		}
		out[ref.Num()] = i
	}
	return out
}

// catalogDestNames returns the names registered in the catalog's
// /Dests dictionary. Used to assert which anchors actually made it into
// the document. Returns nil if the catalog has no /Dests.
func catalogDestNames(t *testing.T, data []byte) []string {
	t.Helper()
	r := parsePDF(t, data)
	cat := r.Catalog()
	destsObj := cat.Get("Dests")
	if destsObj == nil {
		return nil
	}
	resolved, err := r.ResolveObject(destsObj)
	if err != nil {
		t.Fatalf("resolve /Dests: %v", err)
	}
	d, ok := resolved.(*core.PdfDictionary)
	if !ok {
		t.Fatal("/Dests is not a dictionary")
	}
	var names []string
	for k := range d.All() {
		names = append(names, k)
	}
	return names
}

// pageTextAt extracts text from a single page index. Used to verify that
// a resolved /Dest jumps to a page actually containing the expected
// content — proves the page index, not just a non-zero number.
func pageTextAt(t *testing.T, data []byte, idx int) string {
	t.Helper()
	r := parsePDF(t, data)
	if idx < 0 || idx >= r.PageCount() {
		t.Fatalf("page index %d out of range [0, %d)", idx, r.PageCount())
	}
	p, err := r.Page(idx)
	if err != nil {
		t.Fatalf("Page(%d): %v", idx, err)
	}
	text, err := p.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText page %d: %v", idx, err)
	}
	return text
}

// TestHTMLExternalLinkEmitsURIAnnotation verifies that a plain
// <a href="https://..."> anywhere in HTML produces a real PDF link
// annotation with a /URI action, not just blue underlined text.
func TestHTMLExternalLinkEmitsURIAnnotation(t *testing.T) {
	src := `<p><a href="https://example.com/folio">Open Folio</a></p>`
	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 {
		t.Fatalf("expected 1 link annotation, got %d", len(links))
	}
	if links[0].kind != "URI" {
		t.Errorf("kind = %q, want URI", links[0].kind)
	}
	if links[0].uri != "https://example.com/folio" {
		t.Errorf("uri = %q, want https://example.com/folio", links[0].uri)
	}
}

// TestHTMLInternalAnchorLinkResolvesToPage verifies that an
// <a href="#anchor"> in HTML jumps to the element with id="anchor"
// without requiring the caller to manually call AddNamedDest.
//
// Acceptance for B5: clicking <a href="#totals"> in Preview/Acrobat
// scrolls to the <h2 id="totals"> element's page.
func TestHTMLInternalAnchorLinkResolvesToPage(t *testing.T) {
	// Force the link and the anchor onto separate pages so the dest
	// resolution is unambiguous: a same-page jump would still "work"
	// trivially without proper named-destination wiring.
	src := `<p><a href="#totals">Jump to totals</a></p>` +
		`<div style="page-break-before: always"></div>` +
		`<h2 id="totals">TOTALS_HEADING</h2><p>Final figures.</p>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 {
		t.Fatalf("expected 1 link annotation, got %d", len(links))
	}
	l := links[0]
	if l.kind != "Dest" {
		t.Fatalf("kind = %q, want Dest (auto-registered named dest should resolve to /Dest, not /URI/GoTo)", l.kind)
	}
	if l.destPage < 0 {
		t.Fatalf("destPage unresolved — /Dest pageref does not match any page in the page tree")
	}
	if l.srcPage == l.destPage {
		t.Errorf("link and anchor should be on different pages; both on page %d", l.srcPage)
	}
	// The dest page must actually contain the heading text — this is the
	// real bug surface for the off-by-one fix at document.go's finalPageIdx.
	if !strings.Contains(pageTextAt(t, pdfBytes, l.destPage), "TOTALS_HEADING") {
		t.Errorf("dest page %d does not contain TOTALS_HEADING; got: %q",
			l.destPage, pageTextAt(t, pdfBytes, l.destPage))
	}
	if names := catalogDestNames(t, pdfBytes); len(names) != 1 || names[0] != "totals" {
		t.Errorf("catalog /Dests names = %v, want [totals]", names)
	}
}

// TestHTMLAnchorIdsUniqueAcrossDocument verifies a backwards link from
// later content to an earlier id="..." — proves the dest registration
// is not order-dependent and works regardless of source/target ordering.
func TestHTMLAnchorIdsUniqueAcrossDocument(t *testing.T) {
	src := `<h1 id="intro">INTRO_HEADING</h1><p>Body.</p>` +
		`<div style="page-break-before: always"></div>` +
		`<p><a href="#intro">Back to intro</a></p>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 {
		t.Fatalf("expected 1 link annotation, got %d", len(links))
	}
	l := links[0]
	if l.kind != "Dest" || l.destPage < 0 {
		t.Fatalf("link did not resolve to a known page: %+v", l)
	}
	if l.destPage >= l.srcPage {
		t.Errorf("back-reference should jump to earlier page; src=%d dest=%d", l.srcPage, l.destPage)
	}
	if !strings.Contains(pageTextAt(t, pdfBytes, l.destPage), "INTRO_HEADING") {
		t.Errorf("dest page %d missing INTRO_HEADING", l.destPage)
	}
}

// TestHTMLMultipleAnchorsAcrossPages exercises a more realistic case:
// several id-bearing elements on different pages, each linked from
// elsewhere. Verifies that auto-registration scales beyond a single
// dest and doesn't conflate names across pages.
func TestHTMLMultipleAnchorsAcrossPages(t *testing.T) {
	src := `<p>Table of contents:</p>` +
		`<p><a href="#a">A</a> <a href="#b">B</a> <a href="#c">C</a></p>` +
		`<div style="page-break-before: always"></div>` +
		`<h1 id="a">CHAPTER_A_MARKER</h1><p>Body.</p>` +
		`<div style="page-break-before: always"></div>` +
		`<h1 id="b">CHAPTER_B_MARKER</h1><p>Body.</p>` +
		`<div style="page-break-before: always"></div>` +
		`<h1 id="c">CHAPTER_C_MARKER</h1><p>Body.</p>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 3 {
		t.Fatalf("expected 3 link annotations, got %d", len(links))
	}
	// Each link must resolve to its own anchor's page — uses the markers
	// so we know the wiring isn't accidentally cross-binding (e.g. all
	// three pointing to chapter A would still pass a count-only check).
	want := map[string]string{
		"a": "CHAPTER_A_MARKER",
		"b": "CHAPTER_B_MARKER",
		"c": "CHAPTER_C_MARKER",
	}
	seen := map[int]string{}
	for _, l := range links {
		if l.kind != "Dest" || l.destPage < 0 {
			t.Errorf("link did not resolve cleanly: %+v", l)
			continue
		}
		seen[l.destPage] = pageTextAt(t, pdfBytes, l.destPage)
	}
	for name, marker := range want {
		found := false
		for _, txt := range seen {
			if strings.Contains(txt, marker) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no link resolves to a page containing %q (anchor #%s lost)", marker, name)
		}
	}
	if names := catalogDestNames(t, pdfBytes); len(names) != 3 {
		t.Errorf("catalog /Dests names count = %d (%v), want 3", len(names), names)
	}
}

// TestHTMLMixedExternalAndInternalLinksOnSameLine verifies that a
// paragraph containing both kinds of links produces two distinct
// annotations — neither swallows the other, and they keep their
// respective /URI / /Dest semantics. Regression: a single sameLink
// comparison that ignored DestName previously merged adjacent runs.
func TestHTMLMixedExternalAndInternalLinksOnSameLine(t *testing.T) {
	src := `<p>See <a href="https://example.com">site</a> and <a href="#footer">footer</a>.</p>` +
		`<div style="page-break-before: always"></div>` +
		`<p id="footer">FOOTER_MARKER</p>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 2 {
		t.Fatalf("expected 2 link annotations, got %d (%+v)", len(links), links)
	}
	var uri, dest *parsedLink
	for i := range links {
		switch links[i].kind {
		case "URI":
			uri = &links[i]
		case "Dest":
			dest = &links[i]
		}
	}
	if uri == nil || uri.uri != "https://example.com" {
		t.Errorf("missing URI link or wrong target: %+v", uri)
	}
	if dest == nil || dest.destPage < 0 {
		t.Fatalf("missing or unresolved Dest link: %+v", dest)
	}
	if !strings.Contains(pageTextAt(t, pdfBytes, dest.destPage), "FOOTER_MARKER") {
		t.Errorf("dest page %d does not contain FOOTER_MARKER", dest.destPage)
	}
}

// TestHTMLExternalURLWithFragmentNotTreatedAsInternal verifies that an
// href that contains a fragment but starts with a scheme (e.g.
// "https://example.com/page#section") is routed to /URI, not /Dest.
// Regression: the simple HasPrefix("#") check must only fire when the
// href is purely a fragment, never for full URLs that happen to end
// with one.
func TestHTMLExternalURLWithFragmentNotTreatedAsInternal(t *testing.T) {
	src := `<p><a href="https://example.com/docs#api">API</a></p>`
	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].kind != "URI" {
		t.Errorf("kind = %q, want URI", links[0].kind)
	}
	if links[0].uri != "https://example.com/docs#api" {
		t.Errorf("uri = %q, want full URL with fragment preserved", links[0].uri)
	}
}

// TestHTMLBlockLevelAnchorLinkResolves verifies the *block*-level <a>
// path (handled by html/converter_link.go via layout.NewInternalLink)
// also benefits from auto-registration. The inline and block paths
// converge on the same per-page Anchors mechanism, so a regression in
// one shouldn't silently leave the other half-broken.
func TestHTMLBlockLevelAnchorLinkResolves(t *testing.T) {
	src := `<a href="#summary">Skip to summary</a>` +
		`<div style="page-break-before: always"></div>` +
		`<h2 id="summary">SUMMARY_MARKER</h2>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 || links[0].kind != "Dest" || links[0].destPage < 0 {
		t.Fatalf("expected 1 resolved Dest link, got %+v", links)
	}
	if !strings.Contains(pageTextAt(t, pdfBytes, links[0].destPage), "SUMMARY_MARKER") {
		t.Errorf("dest page does not contain SUMMARY_MARKER")
	}
}

// TestHTMLNestedAnchorIDRegistersDest verifies an id="..." on a nested
// element (inside containers) is still discoverable from a link in a
// sibling subtree. Regression: prepending the Anchor must happen at the
// element where the id lives, not only at top-level walkChildren.
func TestHTMLNestedAnchorIDRegistersDest(t *testing.T) {
	src := `<p><a href="#deep">Go deep</a></p>` +
		`<div style="page-break-before: always"></div>` +
		`<div><div><h3 id="deep">DEEP_MARKER</h3></div></div>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 || links[0].kind != "Dest" || links[0].destPage < 0 {
		t.Fatalf("expected 1 resolved Dest link to nested id, got %+v", links)
	}
	if !strings.Contains(pageTextAt(t, pdfBytes, links[0].destPage), "DEEP_MARKER") {
		t.Errorf("dest page does not contain DEEP_MARKER")
	}
}

// TestHTMLAnchorOnSameLineAsLink verifies that a link and the anchor
// it targets can sit on the same logical line (no page break between
// them) without producing a self-referential or empty annotation.
func TestHTMLAnchorOnSameLineAsLink(t *testing.T) {
	src := `<p id="here"><a href="#here">Self</a> link.</p>`
	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 || links[0].kind != "Dest" {
		t.Fatalf("expected 1 Dest link, got %+v", links)
	}
	if links[0].srcPage != links[0].destPage {
		t.Errorf("self-page anchor should resolve to its own page: src=%d dest=%d",
			links[0].srcPage, links[0].destPage)
	}
}

// TestHTMLEmptyFragmentHrefDoesNotPanic verifies that an href of just
// "#" — a non-functional anchor — is handled gracefully without
// registering a stray empty-named destination or emitting a junk
// annotation. The PDF must remain valid (qpdf clean).
func TestHTMLEmptyFragmentHrefDoesNotPanic(t *testing.T) {
	src := `<p><a href="#">No-op</a></p>`
	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	// An empty fragment should not register a destination keyed on the
	// empty string (which would corrupt the catalog /Dests dictionary).
	for _, n := range catalogDestNames(t, pdfBytes) {
		if n == "" {
			t.Error("empty-string named destination registered for href=\"#\"")
		}
	}
	// And it should not produce a Dest-typed link annotation pointing
	// nowhere; either zero links or a link of a benign kind is acceptable.
	for _, l := range collectLinks(t, pdfBytes) {
		if l.kind == "Dest" && l.destPage < 0 {
			t.Errorf("empty fragment produced a /Dest annotation with no resolvable page: %+v", l)
		}
	}
}

// TestHTMLInternalLinkSpansPageBreak verifies that a long inline
// <a href="#x"> wrapping across multiple lines still emits internal
// link annotations on every line, all carrying DestName. Regression:
// cloneWithWords' sameRun comparison previously did not include
// LinkDestName, so a paragraph split could drop the dest from the
// continuation lines.
func TestHTMLInternalLinkSpansPageBreak(t *testing.T) {
	long := strings.Repeat("Jump to the totals section ", 30)
	src := `<p><a href="#totals">` + long + `</a></p>` +
		`<div style="page-break-before: always"></div>` +
		`<h2 id="totals">TOTALS_MARKER</h2>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) < 2 {
		t.Fatalf("expected multiple link annotations for wrapped link, got %d", len(links))
	}
	// Every annotation must be a /Dest link to the same page — no /URI
	// leak, no unresolved dest from a continuation line.
	wantDestPage := -1
	for _, l := range links {
		if l.kind != "Dest" {
			t.Errorf("link of kind %q on continuation line — wrapping dropped DestName", l.kind)
			continue
		}
		if l.destPage < 0 {
			t.Errorf("link does not resolve: %+v", l)
			continue
		}
		if wantDestPage < 0 {
			wantDestPage = l.destPage
		} else if l.destPage != wantDestPage {
			t.Errorf("link destPage = %d, want %d (all wrapped lines should target the same anchor)",
				l.destPage, wantDestPage)
		}
	}
	if wantDestPage >= 0 && !strings.Contains(pageTextAt(t, pdfBytes, wantDestPage), "TOTALS_MARKER") {
		t.Errorf("wrapped link's dest page does not contain TOTALS_MARKER")
	}
}

// TestHTMLAnchorAfterManualPages is the regression for the
// document.go off-by-one: when manual pages precede the rendered HTML
// pages, the named destination's PageIndex was double-counting the
// manual pages and pointing past the end (or to a wrong page). The
// fix collapses finalPageIdx to len(all), which already includes the
// copied manual pages. Without manual pages this bug is invisible —
// hence this dedicated test.
func TestHTMLAnchorAfterManualPages(t *testing.T) {
	res, err := html.ConvertFull(
		`<h2 id="totals">TOTALS_MARKER</h2><p>Final figures.</p>`, nil)
	if err != nil {
		t.Fatalf("html.ConvertFull: %v", err)
	}
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetMargins(layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	// Two manually added pages BEFORE the layout-rendered ones. Add a
	// link from page 0 that targets the auto-registered "totals" anchor —
	// when the off-by-one bug was active, the dest pointed to a page
	// past the document's last index and resolution silently failed.
	p1 := doc.AddPage()
	p1.AddText("MANUAL_PAGE_ONE", font.Helvetica, 12, 72, 720)
	p1.AddInternalLink([4]float64{72, 700, 200, 720}, "totals")
	doc.AddPage().AddText("MANUAL_PAGE_TWO", font.Helvetica, 12, 72, 720)
	for _, e := range res.Elements {
		doc.Add(e)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	pdfBytes := buf.Bytes()
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	var hit *parsedLink
	for i := range links {
		if links[i].kind == "Dest" && links[i].srcPage == 0 {
			hit = &links[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("no /Dest link from page 0; links=%+v", links)
	}
	if hit.destPage < 0 {
		t.Fatal("link from manual page 0 did not resolve — named dest points outside the page range (off-by-one)")
	}
	if !strings.Contains(pageTextAt(t, pdfBytes, hit.destPage), "TOTALS_MARKER") {
		t.Errorf("dest page %d does not contain TOTALS_MARKER (got %q) — off-by-one regression",
			hit.destPage, pageTextAt(t, pdfBytes, hit.destPage))
	}
}

// TestHTMLDuplicateIDsResolveToFirst verifies the contract for HTML
// documents that contain the same id twice. Auto-registration keeps
// the FIRST occurrence so behaviour is consistent across both
// resolution paths (inline annotation /Dest array AND catalog /Dests
// dictionary). Browsers do the same.
func TestHTMLDuplicateIDsResolveToFirst(t *testing.T) {
	src := `<h2 id="dup">FIRST_MARKER</h2>` +
		`<div style="page-break-before: always"></div>` +
		`<h2 id="dup">SECOND_MARKER</h2>` +
		`<div style="page-break-before: always"></div>` +
		`<p><a href="#dup">jump</a></p>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 || links[0].kind != "Dest" || links[0].destPage < 0 {
		t.Fatalf("expected 1 resolved Dest link, got %+v", links)
	}
	got := pageTextAt(t, pdfBytes, links[0].destPage)
	if !strings.Contains(got, "FIRST_MARKER") {
		t.Errorf("duplicate-id link should resolve to FIRST occurrence; landed on page containing %q", got)
	}
	// Catalog should not duplicate the entry — Dests is a dict, but we
	// also avoid registering twice in namedDests so the contract holds.
	names := catalogDestNames(t, pdfBytes)
	count := 0
	for _, n := range names {
		if n == "dup" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("catalog /Dests contains %d entries for \"dup\", want 1", count)
	}
}

// TestHTMLAnchorOnAbsolutePositioned verifies that an id on a
// position:absolute / position:fixed element still registers a named
// destination. Without the converter fix, the absolute branch returned
// nil and the id was silently dropped.
func TestHTMLAnchorOnAbsolutePositioned(t *testing.T) {
	src := `<p><a href="#abs">jump</a></p>` +
		`<div style="page-break-before: always"></div>` +
		`<div id="abs" style="position: absolute; top: 100pt; left: 100pt">ABS_MARKER</div>` +
		`<p>FLOW_MARKER</p>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 || links[0].kind != "Dest" || links[0].destPage < 0 {
		t.Fatalf("expected 1 resolved Dest link to absolute id, got %+v", links)
	}
	// The absolute element with id is registered against the surrounding
	// flow's page. We don't assert ABS_MARKER is on the dest page — the
	// absolute is placed page-globally — but the dest must resolve to a
	// real page (i.e. a non-negative index after the page break).
	if links[0].srcPage == links[0].destPage {
		t.Errorf("expected dest page after the page-break, got src=%d dest=%d",
			links[0].srcPage, links[0].destPage)
	}
}

// TestHTMLAnchorWithPageBreakBeforeLandsOnNewPage is the regression for
// the converter ordering bug: the Anchor element was being prepended
// BEFORE the page-break-before, so the dest registered on the previous
// page. After the fix, Anchor sits AFTER the break and lands on the
// new page where the element actually renders.
func TestHTMLAnchorWithPageBreakBeforeLandsOnNewPage(t *testing.T) {
	src := `<p>FIRST_PAGE_MARKER</p>` +
		`<p><a href="#after">jump</a></p>` +
		`<h2 id="after" style="page-break-before: always">SECOND_PAGE_MARKER</h2>`

	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 || links[0].kind != "Dest" || links[0].destPage < 0 {
		t.Fatalf("expected 1 resolved Dest link, got %+v", links)
	}
	// The dest page must contain the heading text — proves the anchor
	// landed on the post-break page, not the pre-break page.
	got := pageTextAt(t, pdfBytes, links[0].destPage)
	if !strings.Contains(got, "SECOND_PAGE_MARKER") {
		t.Errorf("anchor with page-break-before should land on new page; dest page text = %q", got)
	}
	if strings.Contains(got, "FIRST_PAGE_MARKER") {
		t.Errorf("dest page should NOT contain FIRST_PAGE_MARKER; got %q", got)
	}
}

// TestHTMLBlockEmptyFragmentHrefIsTextNotDeadAnnotation verifies that
// a block-level <a href="#"> renders as plain text rather than a
// /Subtype /Link annotation with no /A and no /Dest (which would be a
// silent click target with no behavior).
func TestHTMLBlockEmptyFragmentHrefIsTextNotDeadAnnotation(t *testing.T) {
	// Force the block-level convertLink path: <a> as a direct child of
	// <body>, not nested in a <p>.
	src := `<a href="#">No-op</a>`
	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	for _, l := range collectLinks(t, pdfBytes) {
		t.Errorf("href=\"#\" must not produce any /Subtype /Link annotation; got %+v", l)
	}
	for _, n := range catalogDestNames(t, pdfBytes) {
		if n == "" {
			t.Error("empty-string named destination registered for href=\"#\"")
		}
	}
}

// TestHTMLAnchorOnTableCellResolves verifies that an id on a <td>
// (which the table converter swallows whole, bypassing convertElement)
// still produces a resolvable named destination.
func TestHTMLAnchorOnTableCellResolves(t *testing.T) {
	src := `<p><a href="#cell">jump</a></p>` +
		`<table><tr><td id="cell">CELL_MARKER</td></tr></table>`
	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 {
		t.Fatalf("want 1 link, got %d: %+v", len(links), links)
	}
	if links[0].kind != "Dest" || links[0].destPage < 0 {
		t.Fatalf("link did not resolve to a page: %+v", links[0])
	}
	if !strings.Contains(pageTextAt(t, pdfBytes, links[0].destPage), "CELL_MARKER") {
		t.Errorf("dest page should contain CELL_MARKER; got %q",
			pageTextAt(t, pdfBytes, links[0].destPage))
	}
	names := catalogDestNames(t, pdfBytes)
	found := false
	for _, n := range names {
		if n == "cell" {
			found = true
		}
	}
	if !found {
		t.Errorf("catalog /Dests missing \"cell\"; got %v", names)
	}
}

// TestHTMLAnchorOnListItemResolves covers id on <li>.
func TestHTMLAnchorOnListItemResolves(t *testing.T) {
	src := `<p><a href="#step2">jump</a></p>` +
		`<ol><li id="step1">first</li><li id="step2">STEP2_MARKER</li></ol>`
	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 || links[0].kind != "Dest" || links[0].destPage < 0 {
		t.Fatalf("want 1 resolved Dest link, got %+v", links)
	}
	if !strings.Contains(pageTextAt(t, pdfBytes, links[0].destPage), "STEP2_MARKER") {
		t.Errorf("dest page should contain STEP2_MARKER; got %q",
			pageTextAt(t, pdfBytes, links[0].destPage))
	}
}

// TestHTMLAnchorOnInlineSpanResolves covers id on <span> inside a <p>
// (the collectRunsFromNode path).
func TestHTMLAnchorOnInlineSpanResolves(t *testing.T) {
	src := `<p><a href="#mid">jump</a></p>` +
		`<p>before <span id="mid">SPAN_MARKER</span> after</p>`
	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 || links[0].kind != "Dest" || links[0].destPage < 0 {
		t.Fatalf("want 1 resolved Dest link, got %+v", links)
	}
	if !strings.Contains(pageTextAt(t, pdfBytes, links[0].destPage), "SPAN_MARKER") {
		t.Errorf("dest page should contain SPAN_MARKER; got %q",
			pageTextAt(t, pdfBytes, links[0].destPage))
	}
}

// TestHTMLAnchorAnnotationAndCatalogAgree verifies that for a destination
// reached via the inline /Dest array (from the link annotation) and the
// same name in the catalog /Dests dictionary, both resolve to the same
// page. This catches regressions where dedup behavior on one path
// diverges from the other.
func TestHTMLAnchorAnnotationAndCatalogAgree(t *testing.T) {
	src := `<p><a href="#totals">jump</a></p>` +
		`<div style="page-break-before: always"></div>` +
		`<h2 id="totals">TOTALS_MARKER</h2>`
	pdfBytes, _ := renderHTMLToPDF(t, src)
	qpdfCheck(t, pdfBytes)

	links := collectLinks(t, pdfBytes)
	if len(links) != 1 || links[0].kind != "Dest" || links[0].destPage < 0 {
		t.Fatalf("want 1 resolved Dest link, got %+v", links)
	}
	annotPage := links[0].destPage

	// Walk the catalog /Dests entry by name and confirm it resolves to
	// the same page as the inline /Dest array.
	r := parsePDF(t, pdfBytes)
	pageByRef := buildPageNumByRef(t, r)
	cat := r.Catalog()
	destsObj := cat.Get("Dests")
	if destsObj == nil {
		t.Fatal("/Dests missing in catalog")
	}
	resolved, err := r.ResolveObject(destsObj)
	if err != nil {
		t.Fatalf("resolve /Dests: %v", err)
	}
	dd, ok := resolved.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("/Dests not a dict, got %T", resolved)
	}
	entry := dd.Get("totals")
	if entry == nil {
		t.Fatal("/Dests missing \"totals\" entry")
	}
	entryArr, ok := entry.(*core.PdfArray)
	if !ok {
		// May be wrapped in an indirect ref pointing at an array.
		if er, err := r.ResolveObject(entry); err == nil {
			entryArr, _ = er.(*core.PdfArray)
		}
	}
	if entryArr == nil || entryArr.Len() < 1 {
		t.Fatalf("/Dests \"totals\" not a usable array: %T %v", entry, entry)
	}
	ref, ok := entryArr.At(0).(*core.PdfIndirectReference)
	if !ok {
		t.Fatalf("/Dests \"totals\" first entry not an indirect ref: %T", entryArr.At(0))
	}
	catalogPage, ok := pageByRef[ref.Num()]
	if !ok {
		t.Fatalf("/Dests page ref %d not in page tree", ref.Num())
	}
	if catalogPage != annotPage {
		t.Errorf("annotation /Dest page = %d, catalog /Dests page = %d; the two resolution paths disagree",
			annotPage, catalogPage)
	}
}

// renderHTMLToPDF runs the full html → document → PDF pipeline and
// returns both the raw PDF bytes (for qpdf validation) and the same
// data as a string (for legacy substring assertions in older tests).
// New tests should use collectLinks / pageTextAt instead.
func renderHTMLToPDF(t *testing.T, src string) ([]byte, string) {
	t.Helper()

	res, err := html.ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("html.ConvertFull: %v", err)
	}

	doc := document.NewDocument(document.PageSizeA4)
	doc.SetMargins(layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	for _, e := range res.Elements {
		doc.Add(e)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("doc.WriteTo: %v", err)
	}
	data := buf.Bytes()
	return data, string(data)
}
