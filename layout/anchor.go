// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// Anchor is a zero-height marker element that records a named destination
// at its location in the layout flow. The renderer surfaces the anchor
// on the PageResult of whichever page it lands on, so the document layer
// can register it as a PDF named destination without requiring callers
// to track which page each id="..." element ended up on.
type Anchor struct {
	name string
}

// NewAnchor creates a zero-height marker that registers the given name
// as a PDF named destination on the page where it is laid out.
func NewAnchor(name string) *Anchor {
	return &Anchor{name: name}
}

// Name returns the destination name carried by this anchor.
func (a *Anchor) Name() string { return a.name }

// PlanLayout implements Element. Returns a single zero-height block
// that carries the anchor name. The block has no Draw closure, so it
// emits no PDF operators — it exists solely so render_plans can capture
// (Anchor, PageIndex) into PageResult.Anchors.
func (a *Anchor) PlanLayout(area LayoutArea) LayoutPlan {
	if a.name == "" {
		return LayoutPlan{Status: LayoutFull}
	}
	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: 0,
		Blocks: []PlacedBlock{{
			Width:  0,
			Height: 0,
			Anchor: a.name,
		}},
	}
}
