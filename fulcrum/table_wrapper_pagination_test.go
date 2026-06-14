// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"fmt"
	"strings"
	"testing"
)

// TestLayoutWrapperTablePaginates pins the v0.9.1-fulcrum.12 patch: a
// borderless single-row/single-cell layout-wrapper <table> must not trap its
// body in one un-splittable cell.
//
// The Job/WorkOrder traveler templates wrap their entire body in such a table
// (a <thead> spacer reserves per-page header space; the body lives in one
// <tbody><tr><td>). folio's table paginator splits only between rows, so the
// single giant cell overflowed page 1 and everything after was clipped —
// WorkOrder rendered ~26% of its content on one page vs Chrome's four.
//
// The converter (convertTable) now detects a chrome-less single-row/single-cell
// table and emits the cell's content as normal block flow, which folio
// paginates correctly. Real multi-row tables, and 1x1 tables whose table or
// cell carries a visible box (border/background/border-radius), are left as
// tables — see the border-radius tests in the html package.
func TestLayoutWrapperTablePaginates(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<html><head><style>.row{padding:8pt;border-bottom:1px solid #ccc;font-size:12px}</style></head><body>`)
	// Layout-wrapper table: <thead> spacer + a single <tbody> cell holding far
	// more than one page of block rows.
	b.WriteString(`<table><thead><tr><td><div style="height:1pt"></div></td></tr></thead><tbody><tr><td>`)
	const n = 40
	// Single-token markers (no spaces) so each renders as one Tj show that
	// findTextY can match exactly.
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `<div class="row">wraprow%03d</div>`, i)
	}
	b.WriteString(`</td></tr></tbody></table></body></html>`)

	pdf, err := renderHTMLToPDF(b.String())
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if pages := pageCount(pdf); pages < 2 {
		t.Fatalf("expected >=2 pages — wrapper-table body was trapped in one un-splittable cell, got %d", pages)
	}
	// A late row must still be present — proves the body flowed to later pages
	// instead of being clipped at the page-1 boundary.
	if findTextY(pdf, "wraprow040") < 0 {
		t.Error("last wrapper-table row 'wraprow040' missing from output — content past page 1 was dropped")
	}
}
