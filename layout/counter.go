// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strconv"
	"strings"
)

// CounterPagePlaceholder is the deferred sentinel produced by CSS
// `counter(page)` evaluations whose value is not yet known when the
// containing TextRun is built (HTML→Element conversion runs before
// pagination). The placeholder travels through paragraph layout and is
// substituted at content-stream emission with the resolved 1-based page
// index. The same string is emitted by `html/page.go` for `@page`
// margin-box content.
const CounterPagePlaceholder = "{counter(page)}"

// CounterPagesPlaceholder is the deferred sentinel for CSS
// `counter(pages)`. It is substituted with the document's final page
// total once pagination is complete and emission begins.
const CounterPagesPlaceholder = "{counter(pages)}"

// counterMeasureDigits is the digit string used to reserve width for a
// counter placeholder during paragraph measurement. The reserved width
// must be at least as wide as the worst-case substituted value so that
// line breaks decided at layout time remain valid after substitution.
// Four "8" digits cover documents up to 9,999 pages; "8" is chosen as a
// conservative wide-glyph approximation. Documents exceeding this bound
// still render correctly (substitution writes the real digits) but may
// see counter values overflow their reserved trailing whitespace.
const counterMeasureDigits = "8888"

// substituteCounters replaces page-counter placeholders in s with their
// resolved 1-based page index and total page count. Returns s unchanged
// when no placeholder is present.
func substituteCounters(s string, pageIdx, totalPages int) string {
	if !strings.Contains(s, "{counter(") {
		return s
	}
	if strings.Contains(s, CounterPagePlaceholder) {
		s = strings.ReplaceAll(s, CounterPagePlaceholder, strconv.Itoa(pageIdx+1))
	}
	if strings.Contains(s, CounterPagesPlaceholder) {
		s = strings.ReplaceAll(s, CounterPagesPlaceholder, strconv.Itoa(totalPages))
	}
	return s
}

// expandCountersForMeasure replaces page-counter placeholders with a
// fixed-width digit reservation string used only for measurement. The
// returned text is used to compute Word.Width so that line breaks are
// stable across digit-count transitions (page 9 → 10, 99 → 100). The
// real placeholder remains on Word.Text and is substituted at emit
// time.
func expandCountersForMeasure(s string) string {
	if !strings.Contains(s, "{counter(") {
		return s
	}
	s = strings.ReplaceAll(s, CounterPagePlaceholder, counterMeasureDigits)
	s = strings.ReplaceAll(s, CounterPagesPlaceholder, counterMeasureDigits)
	return s
}

// hasCounterPlaceholder reports whether s contains a page-counter
// placeholder needing emit-time substitution.
func hasCounterPlaceholder(s string) bool {
	return strings.Contains(s, CounterPagePlaceholder) || strings.Contains(s, CounterPagesPlaceholder)
}
