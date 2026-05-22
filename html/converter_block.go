// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// textShadowFromStyle converts a CSS text-shadow to a layout.TextShadow.
func textShadowFromStyle(style computedStyle) *layout.TextShadow {
	if style.TextShadow == nil {
		return nil
	}
	return &layout.TextShadow{
		OffsetX: style.TextShadow.OffsetX,
		OffsetY: style.TextShadow.OffsetY,
		Blur:    style.TextShadow.Blur,
		Color:   style.TextShadow.Color,
	}
}

// baselineShiftFromStyle computes the vertical baseline offset in points.
// An explicit numeric value (from CSS baseline-shift or vertical-align with
// a length) takes precedence over keyword values like "super" and "sub".
func baselineShiftFromStyle(style computedStyle) float64 {
	// Explicit baseline-shift value (from CSS baseline-shift property)
	// takes precedence over vertical-align keywords.
	if style.BaselineShiftSet {
		return style.BaselineShiftValue
	}
	switch style.VerticalAlign {
	case "super":
		return style.FontSize * 0.35 // raise by ~35% of font size
	case "sub":
		return -style.FontSize * 0.2 // lower by ~20% of font size
	case "text-top":
		return style.FontSize * 0.25
	case "text-bottom":
		return -style.FontSize * 0.15
	default:
		return 0
	}
}

// cssLengthToUnitValue converts a cssLength to a layout.UnitValue.
// Plain percentages and calc/min/max/clamp expressions that depend on a
// percentage are deferred to layout time so they resolve against the
// actual layout area rather than the converter's containerWidth (which
// does not know about page margins or other late-bound constraints).
// Pure absolute values are resolved immediately to points.
func cssLengthToUnitValue(l *cssLength, containerWidth, fontSize float64) layout.UnitValue {
	if l == nil {
		return layout.Pt(0)
	}
	if l.Unit == "%" {
		return layout.Pct(l.Value)
	}
	if l.dependsOnPercent() {
		return layout.CalcUnit(func(available float64) float64 {
			return l.toPoints(available, fontSize)
		})
	}
	return layout.Pt(l.toPoints(containerWidth, fontSize))
}

// narrowContainerWidth saves the current containerWidth, narrows it based on
// the element's padding/border/width, and returns a restore function.
//
// Padding is resolved against the SAVED prev (the parent's content-box
// width) rather than the freshly-narrowed c.containerWidth — per CSS
// 2.1 §8.4, padding/margin percentages resolve against the containing
// block's width, which for a block element is the parent's content
// box, not the element's own (post-Width) box.
func (c *converter) narrowContainerWidth(style computedStyle) func() {
	prev := c.containerWidth
	if style.Width != nil {
		if w := style.Width.toPoints(c.containerWidth, style.FontSize); w > 0 {
			c.containerWidth = w
		}
	}
	if style.hasPadding() {
		c.containerWidth -= style.PaddingLeftAt(prev) + style.PaddingRightAt(prev)
	}
	if style.hasBorder() {
		c.containerWidth -= style.BorderLeftWidth + style.BorderRightWidth
	}
	if c.containerWidth < 0 {
		c.containerWidth = 0
	}
	return func() { c.containerWidth = prev }
}

// convertBlock wraps children in a Div container.
func (c *converter) convertBlock(n *html.Node, style computedStyle) []layout.Element {
	restore := c.narrowContainerWidth(style)

	// Auto-calculate column count from column-width if column-count is not set.
	if style.ColumnCount <= 1 && style.ColumnWidth > 0 && c.containerWidth > 0 {
		gap := style.ColumnGap
		if gap == 0 {
			gap = 12 // default column gap
		}
		style.ColumnCount = int((c.containerWidth + gap) / (style.ColumnWidth + gap))
		if style.ColumnCount < 1 {
			style.ColumnCount = 1
		}
	}

	// Multi-column container: walk children with segmentation at
	// column-span: all boundaries. A column-spanning child breaks the
	// column flow; content before and after is balanced in independent
	// column segments.
	//
	// TODO: a multicol container does not currently carry its own visual
	// box (background, border, padding) — segments are returned directly
	// as siblings, so any box-decoration on the container element is
	// dropped. To fix, wrap the segments in a Div with applyDivStyles.
	// Pre-existing limitation, not introduced by column-span support.
	if style.ColumnCount > 1 {
		segments := c.buildMulticolSegments(n, style)
		restore()
		if len(segments) > 0 {
			return segments
		}
		// All children rendered to nothing. We must NOT walk children
		// again here: buildMulticolSegments has already invoked
		// convertNode on every child, and any side effects (counter
		// increments, absolute positioning, font registration, etc.)
		// have already fired. Re-walking would double-apply them.
		hasVisualBox := style.Height != nil || style.BackgroundColor != nil ||
			style.hasBorder() || style.hasPadding()
		if !hasVisualBox {
			return nil
		}
		div := layout.NewDiv()
		applyDivStyles(div, style, c.containerWidth)
		if bgImg := c.resolveBackgroundImage(style); bgImg != nil {
			div.SetBackgroundImage(bgImg)
		}
		return []layout.Element{div}
	}

	children := c.walkChildren(n, style)
	restore()

	// Allow empty divs that have visual properties (height, background, border).
	hasVisualBox := style.Height != nil || style.BackgroundColor != nil ||
		style.hasBorder() || style.hasPadding()
	if len(children) == 0 && !hasVisualBox {
		return nil
	}

	// If no box-model properties, skip the Div wrapper.
	hasWidthConstraints := style.Width != nil || style.MaxWidth != nil || style.MinWidth != nil
	hasHeightConstraints := style.Height != nil || style.MinHeight != nil || style.MaxHeight != nil || style.AspectRatio > 0
	hasVisualEffects := style.BorderRadius > 0 || style.BorderRadiusTL > 0 || style.BorderRadiusTR > 0 || style.BorderRadiusBR > 0 || style.BorderRadiusBL > 0 || (style.Opacity > 0 && style.Opacity < 1) || style.Overflow == "hidden"
	hasBoxShadow := len(style.BoxShadows) > 0
	hasOutline := style.OutlineWidth > 0
	hasTransform := style.Transform != "" && strings.ToLower(strings.TrimSpace(style.Transform)) != "none"
	hasBgImage := style.BackgroundImage != ""
	if !style.hasPadding() && !style.hasBorder() && !style.hasMargin() && style.BackgroundColor == nil && !hasWidthConstraints && !hasHeightConstraints && !hasVisualEffects && !hasBoxShadow && !hasOutline && !hasTransform && !hasBgImage {
		return children
	}

	// If any child is an AreaBreak, split into multiple Divs separated
	// by the breaks. AreaBreak only works at the top level (the renderer
	// checks for it by type assertion), so burying one inside a Div
	// would silently suppress the page break.
	if containsAreaBreak(children) {
		return c.splitOnAreaBreaks(children, style)
	}

	div := layout.NewDiv()
	for _, child := range children {
		div.Add(child)
	}
	applyDivStyles(div, style, c.containerWidth)

	// Apply background image if set.
	if bgImg := c.resolveBackgroundImage(style); bgImg != nil {
		div.SetBackgroundImage(bgImg)
	}

	return []layout.Element{div}
}

// containsAreaBreak reports whether any element in the slice is an AreaBreak.
func containsAreaBreak(elems []layout.Element) bool {
	for _, e := range elems {
		if _, ok := e.(*layout.AreaBreak); ok {
			return true
		}
	}
	return false
}

// splitOnAreaBreaks produces a sequence of Divs separated by AreaBreak
// elements. Each Div gets the same styles applied. This ensures AreaBreak
// elements appear at the top level where the renderer can act on them.
func (c *converter) splitOnAreaBreaks(children []layout.Element, style computedStyle) []layout.Element {
	var result []layout.Element
	var group []layout.Element

	flush := func() {
		if len(group) == 0 {
			return
		}
		div := layout.NewDiv()
		for _, child := range group {
			div.Add(child)
		}
		applyDivStyles(div, style, c.containerWidth)
		if bgImg := c.resolveBackgroundImage(style); bgImg != nil {
			div.SetBackgroundImage(bgImg)
		}
		result = append(result, div)
		group = nil
	}

	for _, child := range children {
		if _, ok := child.(*layout.AreaBreak); ok {
			flush()
			result = append(result, child)
		} else {
			group = append(group, child)
		}
	}
	flush()

	return result
}

// applyDivStyles applies common computed style properties to a layout.Div.
// containerWidth is the available width in points, used to resolve percentage values.
func applyDivStyles(div *layout.Div, style computedStyle, containerWidth float64) {
	if style.hasPadding() {
		div.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTopAt(containerWidth),
			Right:  style.PaddingRightAt(containerWidth),
			Bottom: style.PaddingBottomAt(containerWidth),
			Left:   style.PaddingLeftAt(containerWidth),
		})
	}
	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	}
	if mt := style.MarginTopAt(containerWidth); mt > 0 {
		div.SetSpaceBefore(mt)
	}
	if mb := style.MarginBottomAt(containerWidth); mb > 0 {
		div.SetSpaceAfter(mb)
	}
	// Horizontal alignment via auto margins.
	if style.MarginLeftAuto && style.MarginRightAuto {
		div.SetHCenter(true)
	} else if style.MarginLeftAuto && !style.MarginRightAuto {
		div.SetHRight(true)
	}
	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	}
	if style.Width != nil {
		div.SetWidthUnit(cssLengthToUnitValue(style.Width, containerWidth, style.FontSize))
	}
	if style.MaxWidth != nil {
		div.SetMaxWidthUnit(cssLengthToUnitValue(style.MaxWidth, containerWidth, style.FontSize))
	}
	if style.MinWidth != nil {
		div.SetMinWidthUnit(cssLengthToUnitValue(style.MinWidth, containerWidth, style.FontSize))
	}
	if style.Height != nil {
		div.SetHeightUnit(cssLengthToUnitValue(style.Height, containerWidth, style.FontSize))
	}
	if style.MinHeight != nil {
		div.SetMinHeightUnit(cssLengthToUnitValue(style.MinHeight, containerWidth, style.FontSize))
	}
	if style.MaxHeight != nil {
		div.SetMaxHeightUnit(cssLengthToUnitValue(style.MaxHeight, containerWidth, style.FontSize))
	}
	if style.AspectRatio > 0 {
		div.SetAspectRatio(style.AspectRatio)
	}
	if style.BorderRadiusTL > 0 || style.BorderRadiusTR > 0 || style.BorderRadiusBR > 0 || style.BorderRadiusBL > 0 {
		div.SetBorderRadiusPerCorner(style.BorderRadiusTL, style.BorderRadiusTR, style.BorderRadiusBR, style.BorderRadiusBL)
	} else if style.BorderRadius > 0 {
		div.SetBorderRadius(style.BorderRadius)
	}
	if style.Clear != "" && style.Clear != "none" {
		div.SetClear(style.Clear)
	}
	if style.Opacity > 0 && style.Opacity < 1 {
		div.SetOpacity(style.Opacity)
	}
	if style.Overflow == "hidden" {
		div.SetOverflow("hidden")
	}
	for _, bs := range style.BoxShadows {
		div.AddBoxShadow(layout.BoxShadow{
			OffsetX: bs.OffsetX,
			OffsetY: bs.OffsetY,
			Blur:    bs.Blur,
			Spread:  bs.Spread,
			Color:   bs.Color,
		})
	}
	if style.OutlineWidth > 0 {
		div.SetOutline(style.OutlineWidth, style.OutlineStyle, style.OutlineColor, style.OutlineOffset)
	}
	if ops := parseTransform(style.Transform); len(ops) > 0 {
		div.SetTransform(ops)
		// Compute approximate element dimensions for transform-origin.
		// Use maxWidth/width hint if available; otherwise use a default.
		w := 0.0
		if style.Width != nil {
			w = style.Width.toPoints(containerWidth, style.FontSize)
		} else if style.MaxWidth != nil {
			w = style.MaxWidth.toPoints(containerWidth, style.FontSize)
		}
		h := 0.0
		if style.Height != nil {
			h = style.Height.toPoints(containerWidth, style.FontSize)
		} else if style.MinHeight != nil {
			h = style.MinHeight.toPoints(containerWidth, style.FontSize)
		}
		ox, oy := parseTransformOrigin(style.TransformOrigin, w, h, style.FontSize)
		div.SetTransformOrigin(ox, oy)
	}
	if style.PageBreakInside == "avoid" {
		div.SetKeepTogether(true)
	}
}

// buildMulticolSegments walks the direct children of a multi-column parent,
// segmenting the flow at children with column-span: all. Each contiguous run
// of non-spanning children becomes its own layout.Columns element; spanning
// children are emitted between them as full-width siblings. The result is a
// sequence of layout elements that stack vertically in the parent.
//
// Per the CSS Multi-column Layout spec, column-span: all only takes effect
// on direct children of a multicol container, so we inspect only the
// immediate child list (n.FirstChild..NextSibling). A column-span: all
// declaration on a deeper descendant is ignored.
//
// Invariant: this function relies on computeElementStyle being side-effect
// free. The peek below recomputes the child's style purely to detect
// column-span before convertNode runs the conversion (which itself
// recomputes the style as part of normal element handling). If
// computeElementStyle ever acquires side effects (counter increments, font
// registration, etc.) the peek would double-apply them and corrupt state.
//
// TODO: this function does NOT group consecutive inline-flow children
// (text + <strong>/<em>/<span>/<a>) into anonymous block boxes the way
// walkChildren does. Mixed inline/text children of a multicol container
// will produce one paragraph per sibling node instead of one wrapped
// paragraph — the same bug pattern fixed for walkChildren in this PR.
// The fix here would be to add an inline-buffering pass equivalent to
// walkChildren's flushInline helper, gated on isInlineFlowChild. Left as
// a follow-up because multicol containers with mixed inline children are
// uncommon in the reported templates.
func (c *converter) buildMulticolSegments(n *html.Node, style computedStyle) []layout.Element {
	var result []layout.Element
	var segment []layout.Element
	var prevMarginBottom float64

	flushSegment := func() {
		if len(segment) == 0 {
			return
		}
		result = append(result, c.buildColumnsSegment(segment, style))
		segment = nil
		prevMarginBottom = 0
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		// Only element nodes can carry column-span: all. Text nodes
		// (whitespace, content) become regular segment content.
		isSpanAll := false
		if child.Type == html.ElementNode {
			// Pure peek — see invariant note above.
			childStyle := c.computeElementStyle(child, style)
			isSpanAll = childStyle.ColumnSpan == "all"
		}

		childElems := c.convertNode(child, style)
		if len(childElems) == 0 {
			// Child rendered to nothing (display:none, comment,
			// empty text node, absolute-positioned, etc.). Skip
			// without disturbing segments — even if isSpanAll is
			// true, an invisible spanning child must not flush.
			continue
		}

		if isSpanAll {
			flushSegment()
			result = append(result, childElems...)
			continue
		}

		for _, e := range childElems {
			prevMarginBottom = collapseMargins(prevMarginBottom, e)
			segment = append(segment, e)
		}
	}
	flushSegment()
	return result
}

// buildColumnsSegment creates a single layout.Columns element from a slice
// of children, applying gap and column-rule from the parent multicol style.
// Children flow sequentially into column 0 and are redistributed at layout
// time via the balanced-fill algorithm so that column heights are equalized
// while preserving document order (CSS Multi-column §3.4, column-fill:
// balance). See https://github.com/carlos7ags/folio/issues/145.
func (c *converter) buildColumnsSegment(children []layout.Element, style computedStyle) layout.Element {
	cols := layout.NewColumns(style.ColumnCount).SetBalanced(true)
	if style.ColumnGap > 0 {
		cols.SetGap(style.ColumnGap)
	}
	if style.ColumnRuleWidth > 0 {
		cols.SetColumnRule(layout.ColumnRule{
			Width: style.ColumnRuleWidth,
			Color: style.ColumnRuleColor,
			Style: style.ColumnRuleStyle,
		})
	}
	for _, child := range children {
		cols.Add(0, child)
	}
	return cols
}

// borderSide identifies which side of a four-sided border is being
// rendered. Used by buildBorderForSide to pick the light vs dark
// color modulation for the 3D bevel styles (groove/ridge/inset/outset).
type borderSide int

const (
	borderTop borderSide = iota
	borderRight
	borderBottom
	borderLeft
)

// buildCellBorders creates layout.CellBorders from a computed style.
func buildCellBorders(style computedStyle) layout.CellBorders {
	return layout.CellBorders{
		Top:    buildBorderForSide(borderTop, style.BorderTopWidth, style.BorderTopStyle, style.BorderTopColor),
		Right:  buildBorderForSide(borderRight, style.BorderRightWidth, style.BorderRightStyle, style.BorderRightColor),
		Bottom: buildBorderForSide(borderBottom, style.BorderBottomWidth, style.BorderBottomStyle, style.BorderBottomColor),
		Left:   buildBorderForSide(borderLeft, style.BorderLeftWidth, style.BorderLeftStyle, style.BorderLeftColor),
	}
}

// buildBorderForSide creates a single layout.Border from width, style,
// and color, with side awareness for the CSS Backgrounds L3 §4.1 3D
// border styles (groove / ridge / inset / outset). Those styles
// modulate the border color per-side to simulate a beveled or carved
// edge — top/left vs bottom/right get opposite modulation. Folio
// approximates each as a single solid stroke in the modulated color
// (rather than the spec's strict two-half-width split-line bevel) —
// this matches what most PDF and browser-print pipelines do, and is
// visually indistinguishable for thin borders. For wider borders the
// bevel is less pronounced than a real browser would draw.
func buildBorderForSide(side borderSide, width float64, style string, color layout.Color) layout.Border {
	if width <= 0 {
		return layout.Border{}
	}
	switch style {
	case "dashed":
		return layout.DashedBorder(width, color)
	case "dotted":
		return layout.DottedBorder(width, color)
	case "double":
		return layout.DoubleBorder(width, color)
	case "groove", "inset":
		return layout.SolidBorder(width, beveledColor(side, color, true))
	case "ridge", "outset":
		return layout.SolidBorder(width, beveledColor(side, color, false))
	default:
		return layout.SolidBorder(width, color)
	}
}

// beveledColor picks the per-side color for a 3D border style.
//
// The `sunken` flag distinguishes the carved-into-surface look
// (groove / inset) from the raised look (ridge / outset). Per CSS
// Backgrounds L3 §4.1:
//
//   - groove / inset: top + left appear dark, bottom + right appear
//     light (light source from bottom-right).
//   - ridge / outset: top + left appear light, bottom + right appear
//     dark (light source from top-left).
//
// Modulation is a fixed 30% lighten / darken from the source color in
// linear sRGB; close enough to the spec's "based on the foreground
// color" guidance and matches the visual weight of typical legacy
// CSS that uses these styles.
func beveledColor(side borderSide, base layout.Color, sunken bool) layout.Color {
	topLeft := sunken
	if side == borderTop || side == borderLeft {
		if topLeft {
			return darkenColor(base)
		}
		return lightenColor(base)
	}
	if topLeft {
		return lightenColor(base)
	}
	return darkenColor(base)
}

// lightenColor returns the source color shifted toward white by 30%.
// Operates in sRGB space (not linear) — adequate for UI bevels
// where strict colorimetric correctness isn't expected.
func lightenColor(c layout.Color) layout.Color {
	if c.Space == layout.ColorSpaceCMYK {
		// Approximate by reducing K (lightness inverse) by 30%.
		return layout.CMYK(c.C, c.M, c.Y, clamp01(c.K*0.7))
	}
	return layout.RGB(
		clamp01(c.R+(1-c.R)*0.3),
		clamp01(c.G+(1-c.G)*0.3),
		clamp01(c.B+(1-c.B)*0.3),
	)
}

// darkenColor returns the source color shifted toward black by 30%.
func darkenColor(c layout.Color) layout.Color {
	if c.Space == layout.ColorSpaceCMYK {
		return layout.CMYK(c.C, c.M, c.Y, clamp01(c.K+(1-c.K)*0.3))
	}
	return layout.RGB(
		clamp01(c.R*0.7),
		clamp01(c.G*0.7),
		clamp01(c.B*0.7),
	)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// convertBlockquote renders a <blockquote> as an indented block with a left border.
func (c *converter) convertBlockquote(n *html.Node, style computedStyle) []layout.Element {
	children := c.walkChildren(n, style)
	if len(children) == 0 {
		return nil
	}

	div := layout.NewDiv()
	for _, child := range children {
		div.Add(child)
	}

	// Left border: 3pt solid gray.
	gray := layout.RGB(0.6, 0.6, 0.6)
	div.SetBorders(layout.CellBorders{
		Left: layout.SolidBorder(3, gray),
	})
	div.SetPaddingAll(layout.Padding{
		Top:    3,
		Right:  6,
		Bottom: 3,
		Left:   15,
	})
	if mt := style.MarginTopAt(c.containerWidth); mt > 0 {
		div.SetSpaceBefore(mt)
	}
	if mb := style.MarginBottomAt(c.containerWidth); mb > 0 {
		div.SetSpaceAfter(mb)
	}
	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	}
	// Override with any explicit border/padding from CSS.
	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	}
	if style.hasPadding() {
		div.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTopAt(c.containerWidth),
			Right:  style.PaddingRightAt(c.containerWidth),
			Bottom: style.PaddingBottomAt(c.containerWidth),
			Left:   style.PaddingLeftAt(c.containerWidth),
		})
	}

	return []layout.Element{div}
}

// convertDefinitionList converts a <dl> element into a series of term/definition pairs.
func (c *converter) convertDefinitionList(n *html.Node, style computedStyle) []layout.Element {
	div := layout.NewDiv()
	if mt := style.MarginTopAt(c.containerWidth); mt > 0 {
		div.SetSpaceBefore(mt)
	}
	if mb := style.MarginBottomAt(c.containerWidth); mb > 0 {
		div.SetSpaceAfter(mb)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		childStyle := c.computeElementStyle(child, style)

		switch child.DataAtom {
		case atom.Dt:
			// Definition term: bold, no indent.
			text := collectText(child)
			if text == "" {
				continue
			}
			text = applyTextTransform(text, childStyle.TextTransform)
			f := resolveFont(childStyle)
			p := layout.NewParagraph(text, f, childStyle.FontSize)
			p.SetAlign(resolveTextAlign(childStyle))
			p.SetLeading(childStyle.LineHeight)
			div.Add(p)

		case atom.Dd:
			// Definition description: indented.
			children := c.walkChildren(child, childStyle)
			if len(children) == 0 {
				continue
			}
			indent := layout.NewDiv()
			for _, ch := range children {
				indent.Add(ch)
			}
			indent.SetPaddingAll(layout.Padding{Left: childStyle.MarginLeftAt(c.containerWidth)})
			div.Add(indent)

		default:
			// Process other children (e.g. nested <div>).
			elems := c.convertNode(child, style)
			for _, e := range elems {
				div.Add(e)
			}
		}
	}

	return []layout.Element{div}
}

// convertFigure converts a <figure> element, rendering <figcaption> as styled caption.
func (c *converter) convertFigure(n *html.Node, style computedStyle) []layout.Element {
	div := layout.NewDiv()
	if mt := style.MarginTopAt(c.containerWidth); mt > 0 {
		div.SetSpaceBefore(mt)
	}
	if mb := style.MarginBottomAt(c.containerWidth); mb > 0 {
		div.SetSpaceAfter(mb)
	}
	if style.hasPadding() {
		div.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTopAt(c.containerWidth),
			Right:  style.PaddingRightAt(c.containerWidth),
			Bottom: style.PaddingBottomAt(c.containerWidth),
			Left:   style.PaddingLeftAt(c.containerWidth),
		})
	}
	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	}
	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		childStyle := c.computeElementStyle(child, style)

		if child.DataAtom == atom.Figcaption {
			// Render figcaption as italic centered paragraph.
			text := collectText(child)
			if text == "" {
				continue
			}
			text = applyTextTransform(text, childStyle.TextTransform)
			f := resolveFont(childStyle)
			p := layout.NewParagraph(text, f, childStyle.FontSize)
			p.SetAlign(layout.AlignCenter)
			p.SetLeading(childStyle.LineHeight)
			p.SetSpaceBefore(4)
			div.Add(p)
		} else {
			// Other children (e.g. <img>, <pre>, <table>).
			elems := c.convertNode(child, style)
			for _, e := range elems {
				div.Add(e)
			}
		}
	}

	return []layout.Element{div}
}
