// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"sort"
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// isNonVisualFlexChild reports whether a direct flex child with the given tag
// generates no box at all (matching convertElementInner's skip list), so it
// must NOT be synthesized into a zero-size placeholder flex item. Visible
// elements that merely render empty (an empty <div>/<span>) are still flex
// items and are kept.
func isNonVisualFlexChild(a atom.Atom) bool {
	switch a {
	case atom.Script, atom.Style, atom.Link, atom.Title, atom.Meta, atom.Head, atom.Base:
		return true
	default:
		return false
	}
}

// convertFlex converts a display:flex container into a layout.Flex.
func (c *converter) convertFlex(n *html.Node, style computedStyle) []layout.Element {
	// Save the parent's containing-block width before narrowing.
	// Margin / padding percentages on the flex container itself
	// resolve against the containing block (CSS 2.1 §8.4), not
	// against the flex's own post-narrow content box.
	parentContainerWidth := c.containerWidth
	restore := c.narrowContainerWidth(style)
	defer restore()

	flex := layout.NewFlex()

	// Map direction. `row-reverse` / `column-reverse` reverse the main-axis
	// order. folio's Flex doesn't model a reversed axis, but the reversal is
	// equivalent to laying the items out in reverse DOM order with
	// justify-content's start/end flipped (the main-start moves to the far
	// end). We apply that equivalence below: flip the justify mapping here and
	// reverse the child order before adding. `.NET DocGen v3 headers use
	// `.logo.container { display:flex; flex-direction:row-reverse }` so the
	// logo right-aligns; without this it stayed at the left.
	reverse := style.FlexDirection == "row-reverse" || style.FlexDirection == "column-reverse"
	switch style.FlexDirection {
	case "column", "column-reverse":
		flex.SetDirection(layout.FlexColumn)
	default:
		flex.SetDirection(layout.FlexRow)
	}

	// Map justify-content. CSS Box Alignment Level 3 lets `start` /
	// `end` stand in for `flex-start` / `flex-end` on this property —
	// align-items and align-content below already accept the shorthand,
	// and consistency matters: a page authored with `justify-content:
	// end` shouldn't silently fall through to `flex-start`.
	justify := layout.JustifyFlexStart
	switch style.JustifyContent {
	case "flex-start", "start", "left":
		justify = layout.JustifyFlexStart
	case "flex-end", "end", "right":
		justify = layout.JustifyFlexEnd
	case "center":
		justify = layout.JustifyCenter
	case "space-between":
		justify = layout.JustifySpaceBetween
	case "space-around":
		justify = layout.JustifySpaceAround
	case "space-evenly":
		justify = layout.JustifySpaceEvenly
	}
	if reverse {
		// Flip start/end so reversed DOM order packs to the correct edge
		// (space-* distributions are symmetric and unaffected).
		switch justify {
		case layout.JustifyFlexStart:
			justify = layout.JustifyFlexEnd
		case layout.JustifyFlexEnd:
			justify = layout.JustifyFlexStart
		}
	}
	flex.SetJustifyContent(justify)

	// Map align-items.
	switch style.AlignItems {
	case "flex-start", "start":
		flex.SetAlignItems(layout.CrossAlignStart)
	case "flex-end", "end":
		flex.SetAlignItems(layout.CrossAlignEnd)
	case "center":
		flex.SetAlignItems(layout.CrossAlignCenter)
	default:
		flex.SetAlignItems(layout.CrossAlignStretch)
	}

	// Map wrap.
	switch style.FlexWrap {
	case "wrap", "wrap-reverse":
		flex.SetWrap(layout.FlexWrapOn)
	default:
		flex.SetWrap(layout.FlexNoWrap)
	}

	// Map align-content (cross-axis distribution for wrapped lines).
	switch style.AlignContent {
	case "flex-end", "end":
		flex.SetAlignContent(layout.JustifyFlexEnd)
	case "center":
		flex.SetAlignContent(layout.JustifyCenter)
	case "space-between":
		flex.SetAlignContent(layout.JustifySpaceBetween)
	case "space-around":
		flex.SetAlignContent(layout.JustifySpaceAround)
	case "space-evenly":
		flex.SetAlignContent(layout.JustifySpaceEvenly)
	}

	if style.Gap > 0 {
		flex.SetGap(style.Gap)
	}
	if style.ColumnGap > 0 && style.Gap == 0 {
		flex.SetColumnGap(style.ColumnGap)
	}

	if style.hasPadding() {
		flex.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTopAt(parentContainerWidth),
			Right:  style.PaddingRightAt(parentContainerWidth),
			Bottom: style.PaddingBottomAt(parentContainerWidth),
			Left:   style.PaddingLeftAt(parentContainerWidth),
		})
	}
	if style.hasBorder() {
		flex.SetBorders(buildCellBorders(style))
	}
	if style.BackgroundColor != nil {
		flex.SetBackground(*style.BackgroundColor)
	}
	if mt := style.MarginTopAt(parentContainerWidth); mt > 0 {
		flex.SetSpaceBefore(mt)
	}
	if mb := style.MarginBottomAt(parentContainerWidth); mb > 0 {
		flex.SetSpaceAfter(mb)
	}

	// Add children as flex items.
	// Each direct HTML child becomes exactly one flex item, even if
	// convertNode returns multiple layout elements (e.g. text with <br>).
	//
	// Children are collected first and then stable-sorted by the CSS
	// `order` property before being added to the Flex container. The
	// stable sort preserves DOM order for equal `order` values, which
	// matches CSS Flexbox spec behavior. Children without `order`
	// have the default value 0.
	type pendingChild struct {
		order int
		item  *layout.FlexItem // non-nil if this child needs FlexItem wrapping
		elem  layout.Element   // non-nil if this child is added as a plain element
	}
	var pending []pendingChild
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		// Skip whitespace-only text nodes inside flex containers (CSS spec:
		// whitespace-only text in flex containers does not generate flex items).
		if child.Type == html.TextNode {
			if strings.TrimSpace(child.Data) == "" {
				continue
			}
		}

		childStyle := style // default
		if child.Type == html.ElementNode {
			childStyle = c.computeElementStyle(child, style)
		} else {
			// Text-node children don't carry their own CSS, but they
			// must not inherit non-inherited properties from the flex
			// container (notably Order, which would otherwise leak and
			// reorder text-node siblings alongside element siblings).
			childStyle.Order = 0
		}

		// A flex item is blockified (CSS Flexbox §4). When an inline element
		// (e.g. a bare <span>) carries a visible box (background, border,
		// and/or border-radius), the bare inline path drops its padding,
		// border, and radius. Route it through convertBlock so it renders its
		// full box model identically to display:inline-block (issue #329).
		// No-op for spans without a box, which stay bare inline Paragraphs.
		childElems, blockified := c.blockifyInlineBoxChild(child, childStyle)
		if !blockified {
			childElems = c.convertNode(child, style)
		}
		if len(childElems) == 0 {
			// A visible element child that rendered no content is still a
			// flex item per CSS Flexbox §4: it generates a zero-size item
			// box that participates in justify-content distribution. Chrome
			// keeps an empty <div class="last-modified"></div> next to its
			// sibling under `justify-content: space-between`, holding the
			// sibling at the far edge; dropping the empty item collapses the
			// row to flex-start (the v3 commerce footer rendered its
			// "Powered By" block left instead of right). Synthesize a
			// zero-size placeholder so the item count — and thus the
			// distribution — matches the browser. Skip only nodes that
			// generate no box at all: non-element nodes, display:none, and
			// non-visual head elements (script/style/link/title/meta).
			if child.Type != html.ElementNode || childStyle.Display == "none" || isNonVisualFlexChild(child.DataAtom) {
				continue
			}
			childElems = []layout.Element{layout.NewDiv()}
		}

		// Per CSS Flexbox §3, the `float` property has no effect on
		// flex items — a direct flex child with `float: left` lays
		// out as if no float were declared. convertElement
		// unconditionally wraps floated elements in a layout.Float
		// box; unwrap any such wrapping here so the flex item sees
		// the underlying element directly. (Without this unwrap,
		// folio's flex width calculation mis-shrinks the column,
		// turning .NET DocGen's `.three-columns { float: left }`
		// inside a flex `.data-container` into aggressively
		// text-wrapped narrow columns.)
		for i, e := range childElems {
			if f, ok := e.(*layout.Float); ok {
				childElems[i] = f.Content()
			}
		}

		// Wrap multiple elements from a single HTML child into a Div
		// so they form one flex item (matching CSS flex behavior).
		var elem layout.Element
		if len(childElems) == 1 {
			elem = childElems[0]
		} else {
			wrapper := layout.NewDiv()
			for _, ce := range childElems {
				wrapper.Add(ce)
			}
			elem = wrapper
		}

		// CSS width on a flex child acts as flex-basis only when the
		// container's main axis is horizontal (row / row-reverse). In a
		// column flex container, `width` is the cross axis (per the CSS
		// Flexbox spec — `width`/`height` always refer to physical
		// dimensions, while flex-basis tracks the main axis), and using
		// it as basis would silently confuse the main-size distribution
		// with a cross-axis constraint. Without this gate, .NET DocGen
		// v3's `.v3-pdf-details-left { flex-grow: 1; width: 250px }`
		// inside `.v3-info-contain1 { flex-direction: column }` had its
		// 250px width swallowed as a vertical basis, and the Div's own
		// width unit was cleared — so the BILLING box stretched to the
		// full cross-axis instead of being held to 250px.
		isRowDirection := style.FlexDirection != "column" && style.FlexDirection != "column-reverse"
		effectiveBasis := childStyle.FlexBasis
		widthUsedAsBasis := false
		if effectiveBasis == nil && childStyle.Width != nil && isRowDirection {
			effectiveBasis = childStyle.Width
			widthUsedAsBasis = true
		}

		// When CSS width is consumed as flex-basis, clear the Div's own width
		// to prevent double-resolution: the flex algorithm already allocates
		// the correct width, so the Div should fill its flex-allocated area
		// rather than re-resolving the percentage against that area.
		if widthUsedAsBasis {
			if d, ok := elem.(*layout.Div); ok {
				d.ClearWidthUnit()
			}
		}

		// Resolve the child's margins against the flex container's
		// content-box width (the narrowed c.containerWidth). The
		// child's containing block is the flex's content box per
		// CSS Flexbox §4.
		childMT := childStyle.MarginTopAt(c.containerWidth)
		childMR := childStyle.MarginRightAt(c.containerWidth)
		childMB := childStyle.MarginBottomAt(c.containerWidth)
		childML := childStyle.MarginLeftAt(c.containerWidth)

		// Check if child has any margin (including negative) that needs FlexItem handling.
		hasMargins := childMT != 0 || childMB != 0 || childML != 0 || childMR != 0

		// Resolve CSS min-width / max-width to points using the
		// containerWidth that was active when this row was being
		// converted. We pass the resolved values into FlexItem so
		// resolveGrowShrink can clamp the item's main-axis size after
		// the basis-and-grow distribution. Without this, .NET DocGen
		// v3's `.contain-left { flex: 1; min-width: 50%; max-width:
		// 55% }` was being silently grown to its 1/3 share of the row
		// instead of being held to its 50% floor.
		var minMain, maxMain float64
		if childStyle.MinWidth != nil {
			minMain = childStyle.MinWidth.toPoints(c.containerWidth, childStyle.FontSize)
		}
		if childStyle.MaxWidth != nil {
			maxMain = childStyle.MaxWidth.toPoints(c.containerWidth, childStyle.FontSize)
		}
		// Clear min-width/max-width on the inner Div so the constraint
		// isn't applied twice — once by the FlexItem's main-axis clamp
		// (against the row's container width) and again by the Div
		// re-resolving the percentage against its FlexItem-allocated
		// width. The double-resolution on a 300pt row → 150pt allocation
		// would re-clamp `max-width: 55%` to 0.55 * 150 = 82.5pt.
		if minMain > 0 || maxMain > 0 {
			if d, ok := elem.(*layout.Div); ok {
				if minMain > 0 {
					d.ClearMinWidthUnit()
				}
				if maxMain > 0 {
					d.ClearMaxWidthUnit()
				}
			}
		}

		needsItem := childStyle.FlexGrow > 0 || childStyle.FlexShrink != 1 ||
			effectiveBasis != nil || (childStyle.AlignSelf != "" && childStyle.AlignSelf != "auto") ||
			childStyle.MarginTopAuto || childStyle.MarginLeftAuto || hasMargins ||
			minMain > 0 || maxMain > 0
		if needsItem {
			item := layout.NewFlexItem(elem)
			if childStyle.FlexGrow > 0 {
				item.SetGrow(childStyle.FlexGrow)
			}
			if childStyle.FlexShrink != 1 {
				item.SetShrink(childStyle.FlexShrink)
			}
			if effectiveBasis != nil {
				item.SetBasisUnit(cssLengthToUnitValue(effectiveBasis, c.containerWidth, childStyle.FontSize))
			}
			if minMain > 0 {
				item.SetMinMainSize(minMain)
			}
			if maxMain > 0 {
				item.SetMaxMainSize(maxMain)
			}
			switch childStyle.AlignSelf {
			case "flex-start", "start":
				item.SetAlignSelf(layout.CrossAlignStart)
			case "flex-end", "end":
				item.SetAlignSelf(layout.CrossAlignEnd)
			case "center":
				item.SetAlignSelf(layout.CrossAlignCenter)
			case "stretch":
				item.SetAlignSelf(layout.CrossAlignStretch)
			}
			if childStyle.MarginTopAuto {
				item.SetMarginTopAuto()
			}
			if childStyle.MarginLeftAuto {
				item.SetMarginLeftAuto()
			}
			if hasMargins {
				item.SetMargins(childMT, childMR, childMB, childML)
				// Clear SpaceBefore/SpaceAfter on the element since the FlexItem's
				// margins handle vertical spacing — otherwise margins are doubled.
				if f, ok := elem.(*layout.Flex); ok {
					f.SetSpaceBefore(0)
					f.SetSpaceAfter(0)
				} else if d, ok := elem.(*layout.Div); ok {
					d.SetSpaceBefore(0)
					d.SetSpaceAfter(0)
				} else if p, ok := elem.(*layout.Paragraph); ok {
					p.SetSpaceBefore(0)
					p.SetSpaceAfter(0)
				}
			}
			pending = append(pending, pendingChild{order: childStyle.Order, item: item})
		} else {
			pending = append(pending, pendingChild{order: childStyle.Order, elem: elem})
		}
	}

	// Stable sort by order so DOM order is preserved for equal values.
	sort.SliceStable(pending, func(i, j int) bool {
		return pending[i].order < pending[j].order
	})
	// row-reverse / column-reverse: reverse the main-axis order (after the
	// `order` sort). Combined with the flipped justify-content above, this
	// reproduces the reversed-axis layout.
	if reverse {
		for i, j := 0, len(pending)-1; i < j; i, j = i+1, j-1 {
			pending[i], pending[j] = pending[j], pending[i]
		}
	}
	for _, p := range pending {
		if p.item != nil {
			flex.AddItem(p.item)
		} else {
			flex.Add(p.elem)
		}
	}

	// Wrap in a Div if the flex container has box-model properties
	// that the Flex type doesn't support (border-radius, opacity, etc.).
	hasExtraVisuals := style.BorderRadius > 0 || style.BorderRadiusTL > 0 || style.BorderRadiusTR > 0 || style.BorderRadiusBR > 0 || style.BorderRadiusBL > 0 ||
		(style.Opacity > 0 && style.Opacity < 1) ||
		style.Overflow == "hidden" ||
		len(style.BoxShadows) > 0 ||
		style.Width != nil || style.MaxWidth != nil || style.MinWidth != nil ||
		isDefiniteHeight(style.Height) || style.MinHeight != nil || style.MaxHeight != nil
	if hasExtraVisuals {
		div := layout.NewDiv()
		// Clear layout properties from the Flex — they'll be applied to the
		// wrapper Div instead. Without this, padding/borders/margins would be
		// applied twice (once on the Flex, once on the Div).
		// Background is kept on BOTH: the Div's background fills the full
		// height/min-height area, while the Flex's background covers content.
		// Since they're the same color, this ensures min-height backgrounds
		// work correctly (matching CSS behavior).
		flex.SetPaddingAll(layout.Padding{})
		flex.SetBorders(layout.CellBorders{})
		flex.SetSpaceBefore(0)
		flex.SetSpaceAfter(0)
		// If the wrapper Div has explicit height, tell the Flex its cross-axis
		// is definite so cross-axis stretching works correctly.
		if isDefiniteHeight(style.Height) {
			flex.SetDefiniteCrossSize(true)
		}
		div.Add(flex)
		// The flex IS the box; when the wrapper Div is height-stretched (an
		// explicit height, or a grid/flex cell stretched to its track), the flex
		// must fill it too so its align-items can position content over the full
		// height (e.g. the v3 index badge centering its number vertically).
		div.SetFillChildHeight(true)
		// applyDivStyles needs the parent's containing-block width
		// for margin/padding percent resolution, not the post-narrow
		// flex content box.
		applyDivStyles(div, style, parentContainerWidth)
		return []layout.Element{div}
	}

	return []layout.Element{flex}
}
