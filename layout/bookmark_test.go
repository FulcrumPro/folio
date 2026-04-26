// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "testing"

// fakeElement is a minimal Element used to drive BookmarkAnchor in
// isolation. It produces a single PlacedBlock and, when splitWidth is
// set, returns LayoutPartial with a smaller continuation copy of itself
// — enough to exercise the overflow-wrap path without pulling in
// Paragraph / Heading.
type fakeElement struct {
	width  float64
	height float64
	split  bool // emit LayoutPartial with a continuation copy
}

func (f *fakeElement) PlanLayout(_ LayoutArea) LayoutPlan {
	plan := LayoutPlan{
		Status:   LayoutFull,
		Consumed: f.height,
		Blocks:   []PlacedBlock{{Width: f.width, Height: f.height}},
	}
	if f.split {
		plan.Status = LayoutPartial
		plan.Overflow = &fakeElement{width: f.width, height: f.height}
	}
	return plan
}

type measurableFake struct {
	fakeElement
	min, max float64
}

func (m *measurableFake) MinWidth() float64 { return m.min }
func (m *measurableFake) MaxWidth() float64 { return m.max }

// TestBookmarkAnchorEmptyLabelKeepsLevelAndClosed exercises the P1 fix:
// a non-empty level with empty label must still write BookmarkLevel and
// BookmarkClosed to Blocks[0]. Render_plans drops empty-text entries on
// its own, so the outline node is suppressed without losing the closed
// intent for downstream debugging.
func TestBookmarkAnchorEmptyLabelKeepsLevelAndClosed(t *testing.T) {
	inner := &fakeElement{width: 100, height: 10}
	a := NewBookmarkAnchor(inner, 2, "", true)
	plan := a.PlanLayout(LayoutArea{Width: 100, Height: 100})
	if got := plan.Blocks[0].BookmarkLevel; got != 2 {
		t.Errorf("BookmarkLevel = %d, want 2 (level must be recorded even without label)", got)
	}
	if !plan.Blocks[0].BookmarkClosed {
		t.Error("BookmarkClosed = false, want true (closed bit must survive empty label)")
	}
	if plan.Blocks[0].HeadingText != "" {
		t.Errorf("HeadingText = %q, want empty (no label was provided)", plan.Blocks[0].HeadingText)
	}
}

// TestBookmarkAnchorOverflowSuppressesEmission verifies that when the
// inner element splits across pages, the overflow is wrapped with a
// "skip" anchor so the second page does not re-emit the bookmark.
func TestBookmarkAnchorOverflowSuppressesEmission(t *testing.T) {
	inner := &fakeElement{width: 100, height: 10, split: true}
	a := NewBookmarkAnchor(inner, 1, "Title", false)
	plan := a.PlanLayout(LayoutArea{Width: 100, Height: 100})

	if plan.Status != LayoutPartial || plan.Overflow == nil {
		t.Fatalf("setup: expected LayoutPartial with Overflow, got status=%v overflow=%v",
			plan.Status, plan.Overflow)
	}

	// First page emits the real bookmark.
	if plan.Blocks[0].HeadingText != "Title" {
		t.Errorf("page 1 HeadingText = %q, want %q", plan.Blocks[0].HeadingText, "Title")
	}
	if plan.Blocks[0].BookmarkLevel != 1 {
		t.Errorf("page 1 BookmarkLevel = %d, want 1", plan.Blocks[0].BookmarkLevel)
	}

	// Continuation must record level=-1 and no HeadingText: the skip
	// sentinel keeps render_plans from re-emitting on the next page.
	contPlan := plan.Overflow.PlanLayout(LayoutArea{Width: 100, Height: 100})
	if contPlan.Blocks[0].BookmarkLevel != -1 {
		t.Errorf("continuation BookmarkLevel = %d, want -1 (skip sentinel)",
			contPlan.Blocks[0].BookmarkLevel)
	}
	if contPlan.Blocks[0].HeadingText != "" {
		t.Errorf("continuation HeadingText = %q, want empty (must not duplicate bookmark)",
			contPlan.Blocks[0].HeadingText)
	}
}

// TestBookmarkAnchorMeasurableConditional verifies that the wrapper
// satisfies Measurable iff its inner does. Wrapping a non-Measurable
// element must NOT silently expose a Measurable that returns 0 — that
// previously caused width collapse in flex / table-cell shrink-to-fit
// callers (the P2 audit finding).
func TestBookmarkAnchorMeasurableConditional(t *testing.T) {
	nonMeasurable := &fakeElement{width: 100, height: 10}
	a := NewBookmarkAnchor(nonMeasurable, 1, "x", false)
	if _, ok := a.(Measurable); ok {
		t.Error("BookmarkAnchor over non-Measurable inner should not satisfy Measurable")
	}

	m := &measurableFake{
		fakeElement: fakeElement{width: 100, height: 10},
		min:         42,
		max:         84,
	}
	a2 := NewBookmarkAnchor(m, 1, "x", false)
	mw, ok := a2.(Measurable)
	if !ok {
		t.Fatal("BookmarkAnchor over Measurable inner should satisfy Measurable")
	}
	if got := mw.MinWidth(); got != 42 {
		t.Errorf("MinWidth = %v, want 42 (must delegate to inner, not return 0)", got)
	}
	if got := mw.MaxWidth(); got != 84 {
		t.Errorf("MaxWidth = %v, want 84 (must delegate to inner, not return 0)", got)
	}
}

// TestBookmarkAnchorEmptyPlanPassthrough is a defensive check: when the
// inner element returns no blocks at all, BookmarkAnchor must not panic
// and must propagate the empty plan unchanged.
func TestBookmarkAnchorEmptyPlanPassthrough(t *testing.T) {
	empty := elementFunc(func(LayoutArea) LayoutPlan {
		return LayoutPlan{Status: LayoutNothing}
	})
	a := NewBookmarkAnchor(empty, 1, "x", false)
	plan := a.PlanLayout(LayoutArea{Width: 100, Height: 100})
	if len(plan.Blocks) != 0 || plan.Status != LayoutNothing {
		t.Errorf("empty inner plan: got %+v, want empty Blocks with LayoutNothing", plan)
	}
}

type elementFunc func(LayoutArea) LayoutPlan

func (f elementFunc) PlanLayout(a LayoutArea) LayoutPlan { return f(a) }
