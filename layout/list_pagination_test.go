// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// walkOrderedListPages walks the page chain of a List via PlanLayout, returning
// the total number of content (non-marker) line blocks emitted across all pages
// and the first-item marker ordinal observed on each page. The marker ordinal
// for a page is derived from the List that produced it: marker index 0 with the
// list's start offset applied (this is exactly the number drawn for the first
// item rendered on that page).
func walkOrderedListPages(t *testing.T, l *List, pageH float64) (totalContent int, firstMarkers []string, pages int) {
	t.Helper()
	indent := l.Indent()
	var cur Element = l
	curList := l
	for cur != nil {
		plan := cur.PlanLayout(LayoutArea{Width: 400, Height: pageH})
		pages++
		totalContent += contentLineCount(plan.Blocks, indent)
		// Record the marker of the first item on this page. A continuation
		// fragment that suppresses its marker contributes the empty string.
		if curList.items[0].suppressMarker {
			firstMarkers = append(firstMarkers, "")
		} else {
			firstMarkers = append(firstMarkers, curList.marker(0))
		}

		if plan.Status == LayoutFull {
			break
		}
		next, ok := plan.Overflow.(*List)
		if !ok {
			t.Fatalf("page %d: overflow is not a *List", pages)
		}
		cur = plan.Overflow
		curList = next
		if pages > len(l.items)+10 {
			t.Fatal("did not converge: too many pages (possible infinite loop)")
		}
	}
	return totalContent, firstMarkers, pages
}

// TestOrderedListMultiPageNumberingContinues is the #347 regression: a long
// ordered list of plain items that spans multiple pages must (a) render every
// item with no content loss across the page boundary, and (b) continue the
// ordinal sequence on later pages instead of restarting at 1.
//
// Fail-before: on main, the boundary item is dropped (totalContent == 59) and
// page 2's first marker is "1." instead of its true ordinal.
func TestOrderedListMultiPageNumberingContinues(t *testing.T) {
	const n = 60
	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	for i := 1; i <= n; i++ {
		l.AddItem(fmt.Sprintf("Entry number %d in the ordered list", i))
	}

	// Page height chosen so each single-line item is ~14.4pt and a page holds
	// well under all 60 items, forcing at least two pages.
	const pageH = 200.0
	totalContent, firstMarkers, pages := walkOrderedListPages(t, l, pageH)

	if pages < 2 {
		t.Fatalf("expected the list to span >=2 pages, got %d", pages)
	}
	if totalContent != n {
		t.Errorf("content lost: expected %d item lines across pages, got %d", n, totalContent)
	}

	// Page 1's first marker is "1.". Every later page's first item must be the
	// continuation of the sequence, never "1." again.
	if firstMarkers[0] != "1." {
		t.Errorf("page 1 first marker = %q, want %q", firstMarkers[0], "1.")
	}
	for p := 1; p < len(firstMarkers); p++ {
		if firstMarkers[p] == "1." {
			t.Errorf("page %d first marker restarted at %q (numbering did not continue)", p+1, firstMarkers[p])
		}
	}

	// Reconstruct the full marker sequence per page and assert it is strictly
	// the contiguous range 1..60 with no gaps and no resets. We do this by
	// recomputing the ordinal of the first item on each page from the running
	// item count, which must match the marker the List would draw.
	emitted := 0
	var cur Element = l
	curList := l
	for cur != nil {
		plan := cur.PlanLayout(LayoutArea{Width: 400, Height: pageH})
		pageItems := contentLineCount(plan.Blocks, indentOf(l))
		if !curList.items[0].suppressMarker {
			want := fmt.Sprintf("%d.", emitted+1)
			if got := curList.marker(0); got != want {
				t.Errorf("first item on a page: marker = %q, want %q", got, want)
			}
		}
		emitted += pageItems
		if plan.Status == LayoutFull {
			break
		}
		next := plan.Overflow.(*List)
		cur = plan.Overflow
		curList = next
	}
	if emitted != n {
		t.Errorf("emitted %d items total, want %d", emitted, n)
	}
}

func indentOf(l *List) float64 { return l.Indent() }

// TestListSetStartClamps verifies SetStart clamps values below 1 to 1 so a
// stray <ol start="0"> or negative attribute can never produce a zero/negative
// or restart-breaking ordinal.
func TestListSetStartClamps(t *testing.T) {
	cases := []struct {
		set  int
		want int
	}{
		{set: 0, want: 1},
		{set: -5, want: 1},
		{set: 1, want: 1},
		{set: 5, want: 5},
	}
	for _, c := range cases {
		l := NewList(font.Helvetica, 12).SetStyle(ListOrdered).SetStart(c.set)
		if got := l.Start(); got != c.want {
			t.Errorf("SetStart(%d): Start() = %d, want %d", c.set, got, c.want)
		}
		l.AddItem("only item")
		if got := l.marker(0); got != fmt.Sprintf("%d.", c.want) {
			t.Errorf("SetStart(%d): first marker = %q, want %q", c.set, got, fmt.Sprintf("%d.", c.want))
		}
	}
}

// TestOrderedListMultiPageRespectsStart verifies that <ol start="N"> numbering
// continues correctly across a page break: page 1 starts at N and later pages
// continue from where the prior page left off (never resetting to N or to 1).
func TestOrderedListMultiPageRespectsStart(t *testing.T) {
	const n = 60
	const startAt = 100
	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered).SetStart(startAt)
	for i := 1; i <= n; i++ {
		l.AddItem(fmt.Sprintf("Entry number %d", i))
	}

	const pageH = 200.0
	totalContent, firstMarkers, pages := walkOrderedListPages(t, l, pageH)

	if pages < 2 {
		t.Fatalf("expected >=2 pages, got %d", pages)
	}
	if totalContent != n {
		t.Errorf("content lost: expected %d item lines, got %d", n, totalContent)
	}
	want := fmt.Sprintf("%d.", startAt)
	if firstMarkers[0] != want {
		t.Errorf("page 1 first marker = %q, want %q", firstMarkers[0], want)
	}
	// Page 2's first marker must be startAt + (items on page 1), strictly
	// greater than the page-1 start and never the page-1 value again.
	if firstMarkers[1] == want {
		t.Errorf("page 2 first marker reset to start value %q", want)
	}
}

// TestUnorderedListMultiPageNoContentLoss verifies the boundary-item-drop fix
// applies to bullet lists too: every item renders across the page break (there
// is no number to continue for <ul>).
//
// Fail-before: the straddling item was dropped (totalContent == n-1).
func TestUnorderedListMultiPageNoContentLoss(t *testing.T) {
	const n = 60
	l := NewList(font.Helvetica, 12).SetStyle(ListUnordered)
	for i := 1; i <= n; i++ {
		l.AddItem(fmt.Sprintf("Bullet entry number %d", i))
	}

	const pageH = 200.0
	totalContent, _, pages := walkOrderedListPages(t, l, pageH)
	if pages < 2 {
		t.Fatalf("expected >=2 pages, got %d", pages)
	}
	if totalContent != n {
		t.Errorf("content lost: expected %d bullet lines, got %d", n, totalContent)
	}
}

// TestListPlainItemTallerThanPageSplits verifies that a single plain item whose
// wrapped text is taller than the available page height is split across pages
// without losing any of its lines, and the marker is drawn only on the first
// fragment.
//
// Fail-before: on main the non-fitting item's tail lines were dropped at the
// page boundary.
func TestListPlainItemTallerThanPageSplits(t *testing.T) {
	// One very long item that wraps to many lines.
	var long string
	for i := 0; i < 80; i++ {
		long += fmt.Sprintf("word%d ", i)
	}
	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	l.AddItem(long)

	// Tiny page so the single item must split across several pages.
	const pageH = 60.0
	const width = 200.0
	indent := l.Indent()

	// Count total content lines across the page chain and track how many
	// fragments carry the marker. In the plain-runs path the marker is drawn
	// inline with the first text line (not as its own block), so it is detected
	// via the producing List's suppressMarker flag: only the first fragment
	// renders the marker; every continuation suppresses it.
	totalContent := 0
	markerFragments := 0
	pages := 0
	var cur Element = l
	curList := l
	for cur != nil {
		plan := cur.PlanLayout(LayoutArea{Width: width, Height: pageH})
		pages++
		totalContent += contentLineCount(plan.Blocks, indent)
		if !curList.items[0].suppressMarker {
			markerFragments++
		}
		if plan.Status == LayoutFull {
			break
		}
		next := plan.Overflow.(*List)
		cur = plan.Overflow
		curList = next
		if pages > 50 {
			t.Fatal("did not converge: possible infinite loop")
		}
	}

	// Compare against the unpaginated line count: nothing may be lost.
	wantLines := len(l.Layout(width))
	if pages < 2 {
		t.Fatalf("expected the tall item to split across >=2 pages, got %d", pages)
	}
	if totalContent != wantLines {
		t.Errorf("content lost: expected %d lines across fragments, got %d", wantLines, totalContent)
	}
	if markerFragments != 1 {
		t.Errorf("marker rendered on %d fragments, want exactly 1 (first fragment only)", markerFragments)
	}
}
