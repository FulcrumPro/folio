// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestAnchorPlanLayoutEmitsZeroHeightBlock(t *testing.T) {
	a := NewAnchor("totals")
	plan := a.PlanLayout(LayoutArea{Width: 400, Height: 1000})

	if plan.Status != LayoutFull {
		t.Errorf("Status = %v, want LayoutFull", plan.Status)
	}
	if plan.Consumed != 0 {
		t.Errorf("Consumed = %v, want 0 (anchor is a zero-height marker)", plan.Consumed)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(plan.Blocks))
	}
	if plan.Blocks[0].Anchor != "totals" {
		t.Errorf("Block.Anchor = %q, want %q", plan.Blocks[0].Anchor, "totals")
	}
	if plan.Blocks[0].Height != 0 {
		t.Errorf("Block.Height = %v, want 0", plan.Blocks[0].Height)
	}
}

func TestAnchorEmptyNameIsNoop(t *testing.T) {
	a := NewAnchor("")
	plan := a.PlanLayout(LayoutArea{Width: 400, Height: 1000})

	if len(plan.Blocks) != 0 {
		t.Errorf("empty anchor should produce no blocks, got %d", len(plan.Blocks))
	}
}

func TestAnchorRendersIntoPageResultAnchors(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewAnchor("intro"))
	r.Add(NewParagraph("Body text", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if len(pages[0].Anchors) != 1 {
		t.Fatalf("expected 1 anchor on page, got %d", len(pages[0].Anchors))
	}
	if pages[0].Anchors[0].Name != "intro" {
		t.Errorf("Anchor.Name = %q, want intro", pages[0].Anchors[0].Name)
	}
}

func TestAnchorOnSecondPage(t *testing.T) {
	// Force enough content onto page 1 that the anchor lands on page 2.
	r := NewRenderer(612, 200, Margins{Top: 36, Right: 36, Bottom: 36, Left: 36})
	for range 10 {
		r.Add(NewParagraph("Filler line of text to push the anchor to the next page.", font.Helvetica, 12))
	}
	r.Add(NewAnchor("section2"))
	r.Add(NewParagraph("Section 2 content", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("expected at least 2 pages, got %d", len(pages))
	}

	// The anchor must land on a later page, not page 0.
	foundOn := -1
	for i, p := range pages {
		for _, a := range p.Anchors {
			if a.Name == "section2" {
				foundOn = i
			}
		}
	}
	if foundOn <= 0 {
		t.Errorf("expected section2 anchor on a non-first page, found on page %d", foundOn)
	}
}

// TestLinkSpansEmitsDestName verifies that a Word with LinkDestName
// (set by inline <a href="#x">) produces a LinkArea carrying DestName,
// not URI. Regression test for the inline-anchor routing.
func TestLinkSpansEmitsDestName(t *testing.T) {
	p := NewParagraph("Jump", font.Helvetica, 12)
	p.runs[0].LinkDestName = "totals"
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least 1 line")
	}
	spans := linkSpans(lines[0].Words)
	if len(spans) != 1 {
		t.Fatalf("expected 1 link span, got %d", len(spans))
	}
	if spans[0].DestName != "totals" {
		t.Errorf("DestName = %q, want totals", spans[0].DestName)
	}
	if spans[0].URI != "" {
		t.Errorf("URI = %q, want empty (internal link)", spans[0].URI)
	}
}

// TestLinkSpansSeparatesURIFromDestName ensures consecutive words with
// different link kinds do not collapse into a single span — a URI link
// followed by a dest link must stay distinct.
func TestLinkSpansSeparatesURIFromDestName(t *testing.T) {
	words := []Word{
		{Text: "ext", Width: 30, LinkURI: "https://example.com"},
		{Text: "int", Width: 30, LinkDestName: "totals"},
	}
	spans := linkSpans(words)
	if len(spans) != 2 {
		t.Fatalf("expected 2 distinct link spans, got %d", len(spans))
	}
	if spans[0].URI != "https://example.com" || spans[0].DestName != "" {
		t.Errorf("span[0] = %+v, want URI-only", spans[0])
	}
	if spans[1].DestName != "totals" || spans[1].URI != "" {
		t.Errorf("span[1] = %+v, want DestName-only", spans[1])
	}
}
