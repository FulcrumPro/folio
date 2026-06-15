// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"strings"
	"testing"
)

// TestPageContentClippedToContentBox pins the v0.9.1-fulcrum.31 patch: each
// page's flow content is clipped to the content box (the area inside the @page
// margins), so an element that bleeds into the margin via a negative margin is
// painted only up to the content edge — matching the browser. The .NET DocGen
// v3 `.title-background` plate (`margin-left: -34px`) bled all the way to the
// physical page edge before this; Chrome clips it at the page content margin.
func TestPageContentClippedToContentBox(t *testing.T) {
	pageW, pageH := 612.0, 792.0
	m := Margins{Top: 20, Right: 30, Bottom: 40, Left: 50}
	r := NewRenderer(pageW, pageH, m)
	d := NewDiv()
	d.SetBackground(Color{R: 0, G: 0, B: 1})
	d.SetHeightUnit(Pt(20))
	r.Add(d)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("no pages rendered")
	}
	stream := string(pages[0].Stream.Bytes())

	// The content-box clip is emitted as "<L> <B> <W> <H> re W n" before the
	// flow blocks. Width = pageW-L-R, Height = pageH-T-B, origin (L, B).
	wantRect := fmt.Sprintf("%g %g %g %g re", m.Left, m.Bottom, pageW-m.Left-m.Right, pageH-m.Top-m.Bottom)
	if !strings.Contains(stream, wantRect) {
		t.Errorf("page stream missing content-box clip rect %q\nstream:\n%s", wantRect, stream)
	}
	// The rect must be followed by a clip op (W) — i.e. it clips, not fills.
	if i := strings.Index(stream, wantRect); i >= 0 {
		after := stream[i+len(wantRect):]
		end := 8
		if len(after) < end {
			end = len(after)
		}
		if !strings.Contains(after[:end], "W") {
			t.Errorf("content-box rect is not used as a clip path (no W after re)")
		}
	}
}
