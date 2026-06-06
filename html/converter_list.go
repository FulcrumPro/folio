// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// convertList handles <ul> and <ol> elements, including nested lists.
func (c *converter) convertList(n *html.Node, style computedStyle, ordered bool) []layout.Element {
	stdFont, embFont := c.resolveFontPair(style)
	var list *layout.List
	if embFont != nil {
		list = layout.NewListEmbedded(embFont, style.FontSize)
	} else {
		list = layout.NewList(stdFont, style.FontSize)
	}
	list.SetLeading(style.LineHeight)

	// Apply ::marker pseudo-element styles from <li> children.
	// Check the first <li> for ::marker declarations and apply to the list.
	if c.sheet != nil {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.DataAtom == atom.Li {
				markerDecls := c.sheet.matchingPseudoElementDeclarations(child, "marker")
				for _, d := range markerDecls {
					switch d.property {
					case "color":
						if clr, ok := parseColor(d.value); ok {
							list.SetMarkerColor(clr)
						}
					case "font-size":
						fs := parseFontSize(d.value, style.FontSize)
						if fs > 0 {
							list.SetMarkerFontSize(fs)
						}
					}
				}
				break // only need to check the first <li>
			}
		}
	}

	// Apply list-style-type from CSS, with fallback to ordered/unordered default.
	switch style.ListStyleType {
	case "disc", "":
		if ordered {
			list.SetStyle(layout.ListOrdered)
		} else {
			list.SetStyle(layout.ListUnordered)
		}
	case "circle", "square":
		list.SetStyle(layout.ListUnordered)
	case "decimal", "decimal-leading-zero":
		list.SetStyle(layout.ListOrdered)
	case "lower-roman":
		list.SetStyle(layout.ListOrderedRoman)
	case "upper-roman":
		list.SetStyle(layout.ListOrderedRomanUp)
	case "lower-alpha", "lower-latin":
		list.SetStyle(layout.ListOrderedAlpha)
	case "upper-alpha", "upper-latin":
		list.SetStyle(layout.ListOrderedAlphaUp)
	case "none":
		list.SetStyle(layout.ListNone)
	default:
		if ordered {
			list.SetStyle(layout.ListOrdered)
		}
	}

	// Propagate text direction to the list so markers position correctly
	// and item paragraphs inherit the direction for bidi reordering.
	if style.Direction != layout.DirectionAuto {
		list.SetDirection(style.Direction)
	}

	c.populateList(n, list, style)

	return []layout.Element{list}
}

// populateList fills a list with items from <li> children, handling nesting.
//
// Each <li> takes one of two paths:
//
//   - Fast (inline) path: the <li> has no box-model styles and contains only
//     inline content (optionally followed by a nested <ul>/<ol>). The item
//     renders as styled TextRuns with the marker inline, exactly as before.
//     Uses collectListItemRuns so inline elements like <a href="..."> are
//     preserved as styled TextRuns with LinkURI.
//
//   - Element path: the <li> has block-level flow children (<div>, <p>, <br>,
//     display:block) and/or its own box-model styles. The <li>'s children are
//     run through the normal block-flow converter (walkChildren) and wrapped
//     in a Div, so block children flow onto separate lines and any
//     background/border/border-radius/padding on the <li> is painted. The
//     marker is aligned to the first text line of the element.
func (c *converter) populateList(n *html.Node, list *layout.List, style computedStyle) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.DataAtom != atom.Li {
			continue
		}

		liStyle := c.computeElementStyle(child, style)
		hasBox := liHasBoxModel(liStyle)
		nestedList := findNestedList(child)

		// Fast path: plain inline item (optionally with a nested list as a
		// sub-list) and no box-model styles. Preserves existing rendering
		// and indentation for the common case.
		if !hasBox && !liHasBlockFlowChildren(c, child, liStyle) {
			runs := c.collectListItemRuns(child, style)
			if nestedList != nil {
				if len(runs) == 0 {
					runs = []layout.TextRun{{Text: " ", Font: font.Helvetica, FontSize: style.FontSize}}
				}
				sub := list.AddItemRunsWithSubList(runs)
				if nestedList.DataAtom == atom.Ol {
					sub.SetStyle(layout.ListOrdered)
				}
				c.populateList(nestedList, sub, style)
			} else if len(runs) > 0 {
				list.AddItemRuns(runs)
			}
			continue
		}

		// Element path: convert the <li>'s children via the normal
		// block-flow path so block elements, <br>, and nested lists lay
		// out correctly. Apply the <li>'s own box styles when present.
		c.addElementListItem(child, list, liStyle, hasBox)
	}
}

// addElementListItem converts an <li> with block-level children and/or
// box-model styles into a rich-element list item.
func (c *converter) addElementListItem(li *html.Node, list *layout.List, liStyle computedStyle, hasBox bool) {
	children := c.walkChildren(li, liStyle)
	if len(children) == 0 {
		// Nothing renderable; still emit an (empty) marker for an empty <li>.
		list.AddItemElement(layout.NewDiv())
		return
	}

	div := layout.NewDiv()
	if hasBox {
		// The element item is laid out in the content column (to the right of
		// the marker), i.e. the available width minus the list indent. Resolve
		// percentage box-model values (padding, border, border-radius) against
		// that narrower width so they match where the box is actually placed
		// rather than overflowing against the full container width.
		contentWidth := c.containerWidth - list.Indent()
		if contentWidth < 0 {
			contentWidth = 0
		}
		applyDivStyles(div, liStyle, contentWidth)

		// The Div now owns and draws the box background (honoring border-radius).
		// Clear the redundant block-level/run background that the <li>'s
		// background propagated onto the inner content paragraph and its text
		// runs, so it does not re-draw a square fill over the rounded corners
		// (same double-paint #340 fixed for convertBlock/blockquote/cells). The
		// content may sit directly in the Div (a badge's single Paragraph) or
		// nested inside an inner wrapper Div (block children), so clear at every
		// depth. The children are cleared before being added below.
		if liStyle.BackgroundColor != nil {
			clearMatchingBackgroundsRecursive(children, *liStyle.BackgroundColor)
		}
	}
	// An inline-block <li> (e.g. a badge "circle around a number") should HUG
	// its content rather than fill the content column. Enable shrink-to-fit
	// (CSS fit-content); an explicit CSS width still wins, since
	// Div.PlanLayout resolves the width unit before applying shrink-to-fit
	// (and applyDivStyles already set the width unit from liStyle.Width).
	if liStyle.Display == "inline-block" {
		div.SetShrinkToFit(true)
	}
	for _, e := range children {
		div.Add(e)
	}
	list.AddItemElement(div)
}

// liHasBoxModel reports whether an <li>'s computed style carries box-model
// decoration that requires a Div wrapper to render (background, border,
// border-radius, or padding).
func liHasBoxModel(s computedStyle) bool {
	return s.hasPadding() || s.hasBorder() || s.hasBorderRadius() ||
		s.BackgroundColor != nil
}

// liHasBlockFlowChildren reports whether an <li> contains any child that
// participates in block flow (block element, <br>, or display:block), which
// requires the block-flow conversion path rather than inline text runs. A
// nested <ul>/<ol> alone does not count: it is handled by the sub-list fast
// path.
func liHasBlockFlowChildren(c *converter, li *html.Node, liStyle computedStyle) bool {
	for child := li.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		// A nested list is handled by the sub-list fast path.
		if child.DataAtom == atom.Ul || child.DataAtom == atom.Ol {
			continue
		}
		if child.DataAtom == atom.Br {
			return true
		}
		if !c.isInlineFlowChild(child, liStyle) {
			return true
		}
	}
	return false
}
