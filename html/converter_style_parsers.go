// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Standalone CSS value parsers extracted from converter_style.go.
// These functions parse CSS property values (transforms, shadows, angles)
// and do not depend on the converter struct.

package html

import (
	"math"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/layout"
)

func parseTransform(val string) []layout.TransformOp {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "none" || val == "" {
		return nil
	}

	var ops []layout.TransformOp
	// Match function calls: name(args)
	for val != "" {
		// Find the next function name.
		parenIdx := strings.Index(val, "(")
		if parenIdx < 0 {
			break
		}
		fname := strings.TrimSpace(val[:parenIdx])
		// Find the matching close paren respecting nesting. A naive
		// strings.Index(val[parenIdx:], ")") returns the FIRST close
		// paren, which truncates mid-expression for inputs like
		// `translate(calc(50% - 10px), 0)` — the calc's inner `)` would
		// be mistaken for the outer function close.
		closeAbs := -1
		depth := 0
		for i := parenIdx; i < len(val); i++ {
			switch val[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					closeAbs = i
				}
			}
			if closeAbs >= 0 {
				break
			}
		}
		if closeAbs < 0 {
			break
		}
		argsStr := val[parenIdx+1 : closeAbs]
		val = strings.TrimSpace(val[closeAbs+1:])

		// Parse arguments. CSS Transforms L1 grammar is comma-separated;
		// browsers also accept legacy space-separated forms. Split on
		// top-level commas first; if that yields a single token, fall
		// back to top-level whitespace splitting for the legacy form.
		// Both helpers respect paren depth, so nested calls like
		// `min(10px, 20px)` survive intact.
		parts := splitTopLevelCommas(argsStr)
		if len(parts) <= 1 {
			parts = splitTopLevelFields(argsStr)
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		switch fname {
		case "rotate":
			if len(parts) >= 1 {
				deg := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "rotate", Values: [2]float64{deg, 0}})
			}
		case "scale":
			if len(parts) >= 2 {
				sx := parseNumericVal(parts[0])
				sy := parseNumericVal(parts[1])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{sx, sy}})
			} else if len(parts) >= 1 {
				s := parseNumericVal(parts[0])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{s, s}})
			}
		case "scalex":
			if len(parts) >= 1 {
				s := parseNumericVal(parts[0])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{s, 1}})
			}
		case "scaley":
			if len(parts) >= 1 {
				s := parseNumericVal(parts[0])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{1, s}})
			}
		case "translate":
			if len(parts) >= 2 {
				tx := parseLengthPx(parts[0])
				ty := parseLengthPx(parts[1])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{tx, -ty}})
			} else if len(parts) >= 1 {
				tx := parseLengthPx(parts[0])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{tx, 0}})
			}
		case "translatex":
			if len(parts) >= 1 {
				tx := parseLengthPx(parts[0])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{tx, 0}})
			}
		case "translatey":
			if len(parts) >= 1 {
				ty := parseLengthPx(parts[0])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{0, -ty}})
			}
		case "skew":
			if len(parts) >= 2 {
				ax := parseAngle(parts[0])
				ay := parseAngle(parts[1])
				ops = append(ops, layout.TransformOp{Type: "skewX", Values: [2]float64{ax, 0}})
				ops = append(ops, layout.TransformOp{Type: "skewY", Values: [2]float64{ay, 0}})
			} else if len(parts) >= 1 {
				ax := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "skewX", Values: [2]float64{ax, 0}})
			}
		case "skewx":
			if len(parts) >= 1 {
				a := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "skewX", Values: [2]float64{a, 0}})
			}
		case "skewy":
			if len(parts) >= 1 {
				a := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "skewY", Values: [2]float64{a, 0}})
			}
		}
	}
	return ops
}

// parseAngle parses a CSS angle value like "45deg", "1.5rad", "100grad",
// "0.5turn", or a calc/min/max/clamp expression containing angle leaves.
// Returns degrees. Calc evaluation is angle-native: every leaf is parsed
// as an angle (or dimensionless number that acts as a multiplier or
// divisor) and arithmetic is applied directly. Mixing length units with
// angles is not meaningful in CSS and yields 0 from the offending leaf.
func parseAngle(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}
	if strings.HasPrefix(s, "calc(") && strings.HasSuffix(s, ")") {
		return resolveAngleCalc(s[5 : len(s)-1])
	}
	if strings.HasPrefix(s, "min(") && strings.HasSuffix(s, ")") {
		parts := splitTopLevelCommas(s[4 : len(s)-1])
		if len(parts) == 0 {
			return 0
		}
		out := parseAngle(parts[0])
		for _, a := range parts[1:] {
			if v := parseAngle(a); v < out {
				out = v
			}
		}
		return out
	}
	if strings.HasPrefix(s, "max(") && strings.HasSuffix(s, ")") {
		parts := splitTopLevelCommas(s[4 : len(s)-1])
		if len(parts) == 0 {
			return 0
		}
		out := parseAngle(parts[0])
		for _, a := range parts[1:] {
			if v := parseAngle(a); v > out {
				out = v
			}
		}
		return out
	}
	if strings.HasPrefix(s, "clamp(") && strings.HasSuffix(s, ")") {
		parts := splitTopLevelCommas(s[6 : len(s)-1])
		if len(parts) != 3 {
			return 0
		}
		lo, mid, hi := parseAngle(parts[0]), parseAngle(parts[1]), parseAngle(parts[2])
		if mid < lo {
			return lo
		}
		if mid > hi {
			return hi
		}
		return mid
	}
	return parseAngleLeaf(s)
}

// resolveAngleCalc evaluates the inside of a calc() over angles. The
// grammar mirrors parseCalcExpr (top-level + and - first; then * and /;
// parenthesised sub-expressions recurse), but every leaf is read as an
// angle in degrees (or a dimensionless number for use as a
// multiplier/divisor). Whitespace around + and - is required by CSS;
// * and / can be tight against their operands.
func resolveAngleCalc(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Top-level + or -, scanned right-to-left for left-associative eval.
	depth := 0
	for i := len(s) - 1; i > 0; i-- {
		switch s[i] {
		case ')':
			depth++
		case '(':
			depth--
		case '+', '-':
			if depth != 0 {
				continue
			}
			// Require surrounding whitespace per CSS spec.
			if i+1 >= len(s) || i-1 < 0 || s[i-1] != ' ' || s[i+1] != ' ' {
				continue
			}
			// Skip "-" that's part of a numeric literal in *... or /... position.
			l := resolveAngleCalc(s[:i])
			r := resolveAngleCalc(s[i+1:])
			if s[i] == '+' {
				return l + r
			}
			return l - r
		}
	}
	// Top-level * or /.
	depth = 0
	for i := len(s) - 1; i > 0; i-- {
		switch s[i] {
		case ')':
			depth++
		case '(':
			depth--
		case '*', '/':
			if depth != 0 {
				continue
			}
			l := resolveAngleCalc(s[:i])
			r := resolveAngleCalc(s[i+1:])
			if s[i] == '*' {
				return l * r
			}
			if r == 0 {
				return 0
			}
			return l / r
		}
	}
	// Parenthesised sub-expression.
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		return resolveAngleCalc(s[1 : len(s)-1])
	}
	// Nested calc/min/max/clamp.
	if strings.HasPrefix(s, "calc(") || strings.HasPrefix(s, "min(") ||
		strings.HasPrefix(s, "max(") || strings.HasPrefix(s, "clamp(") {
		return parseAngle(s)
	}
	return parseAngleLeaf(s)
}

// parseAngleLeaf parses a single angle literal (no calc/min/max/clamp).
// Recognised suffixes: turn, grad, rad, deg. A bare number is treated
// as degrees. Suffix order matters — grad must be checked before rad
// because "100grad" ends in "rad", and turn before either because some
// CSS extensions (Folio doesn't ship them) use "turnxxx" suffixes.
func parseAngleLeaf(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "turn") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "turn"), 64)
		return v * 360
	}
	if strings.HasSuffix(s, "grad") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "grad"), 64)
		return v * 0.9 // 400grad = 360deg
	}
	if strings.HasSuffix(s, "rad") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "rad"), 64)
		return v * 180 / math.Pi
	}
	if strings.HasSuffix(s, "deg") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "deg"), 64)
		return v
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseNumericVal parses a bare numeric value (no unit). Recognises
// calc/min/max/clamp wrappers whose leaves are dimensionless numbers,
// so transforms like `scale(calc(0.5 + 0.5))` and `skew(min(1deg,
// 2))` resolve correctly. min/max/clamp are evaluated directly here
// rather than through parseLength because parseLength's math-arg
// helper tags bare numbers as Unit "px" (the right default for length
// contexts but wrong for dimensionless math).
func parseNumericVal(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	if strings.HasPrefix(s, "min(") && strings.HasSuffix(s, ")") {
		parts := splitTopLevelCommas(s[4 : len(s)-1])
		if len(parts) == 0 {
			return 0
		}
		out := parseNumericVal(parts[0])
		for _, a := range parts[1:] {
			if v := parseNumericVal(a); v < out {
				out = v
			}
		}
		return out
	}
	if strings.HasPrefix(s, "max(") && strings.HasSuffix(s, ")") {
		parts := splitTopLevelCommas(s[4 : len(s)-1])
		if len(parts) == 0 {
			return 0
		}
		out := parseNumericVal(parts[0])
		for _, a := range parts[1:] {
			if v := parseNumericVal(a); v > out {
				out = v
			}
		}
		return out
	}
	if strings.HasPrefix(s, "clamp(") && strings.HasSuffix(s, ")") {
		parts := splitTopLevelCommas(s[6 : len(s)-1])
		if len(parts) != 3 {
			return 0
		}
		lo, mid, hi := parseNumericVal(parts[0]), parseNumericVal(parts[1]), parseNumericVal(parts[2])
		if mid < lo {
			return lo
		}
		if mid > hi {
			return hi
		}
		return mid
	}
	if l := parseLength(s); l != nil && l.isDimensionless() {
		return l.toPoints(0, 0)
	}
	return 0
}

// parseLengthPx parses a CSS length for use in transforms (px → pt conversion).
func parseLengthPx(s string) float64 {
	l := parseLength(s)
	if l != nil {
		return l.toPoints(0, 12) // default font size context
	}
	// Bare number — treat as px.
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v * 0.75
}

// parseTransformOrigin parses a CSS transform-origin value like
// "center center", "top left", "50% 50%" into point coordinates
// relative to the element's top-left corner.
func parseTransformOrigin(val string, width, height, fontSize float64) (float64, float64) {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "" {
		// Default: center center.
		return width / 2, height / 2
	}

	// splitTopLevelFields keeps calc()/min()/max()/clamp() values as a
	// single token even when they contain internal whitespace.
	parts := splitTopLevelFields(val)
	if len(parts) == 1 {
		// Single value: applies to X, Y defaults to center.
		x := resolveOriginComponent(parts[0], width, fontSize)
		return x, height / 2
	}
	x := resolveOriginComponent(parts[0], width, fontSize)
	y := resolveOriginComponent(parts[1], height, fontSize)
	return x, y
}

// resolveOriginComponent resolves a single transform-origin keyword or length
// to a point value relative to the given dimension.
func resolveOriginComponent(s string, dimension, fontSize float64) float64 {
	switch s {
	case "left", "top":
		return 0
	case "center":
		return dimension / 2
	case "right", "bottom":
		return dimension
	default:
		if l := parseLength(s); l != nil {
			return l.toPoints(dimension, fontSize)
		}
		return dimension / 2
	}
}

// parseBoxShadow parses a CSS box-shadow value.
// Format: "offsetX offsetY blur spread color" or "none".
func parseBoxShadow(val string, fontSize float64) *boxShadow {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "none" || val == "" {
		return nil
	}

	// Remove "inset" keyword if present.
	inset := false
	if strings.Contains(val, "inset") {
		inset = true
		val = strings.ReplaceAll(val, "inset", "")
		val = strings.TrimSpace(val)
	}

	// splitTopLevelFields keeps functional values intact (calc/min/max/
	// clamp for the length slots, rgb/rgba/hsl for the color slot) when
	// they contain internal whitespace.
	parts := splitTopLevelFields(val)
	if len(parts) < 2 {
		return nil
	}

	bs := &boxShadow{Inset: inset}

	// Parse lengths (up to 4) and the remaining token as color.
	var lengths []float64
	var colorToken string
	for _, p := range parts {
		if l := parseLength(p); l != nil {
			lengths = append(lengths, l.toPoints(0, fontSize))
		} else {
			// Accumulate as potential color token.
			if colorToken == "" {
				colorToken = p
			} else {
				colorToken += " " + p
			}
		}
	}

	if len(lengths) >= 2 {
		bs.OffsetX = lengths[0]
		bs.OffsetY = lengths[1]
	}
	if len(lengths) >= 3 {
		bs.Blur = lengths[2]
	}
	if len(lengths) >= 4 {
		bs.Spread = lengths[3]
	}

	if colorToken != "" {
		if c, ok := parseColor(colorToken); ok {
			bs.Color = c
		} else {
			bs.Color = layout.ColorBlack
		}
	} else {
		bs.Color = layout.ColorBlack
	}

	return bs
}

// parseBoxShadows parses a CSS box-shadow value that may contain multiple
// comma-separated shadows. Commas inside function calls (e.g. rgba()) are
// not treated as separators.
func parseBoxShadows(val string, fontSize float64) []boxShadow {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "none" || val == "" {
		return nil
	}

	// Split on commas that are not inside parentheses.
	parts := splitTopLevelCommas(val)
	var shadows []boxShadow
	for _, part := range parts {
		if bs := parseBoxShadow(strings.TrimSpace(part), fontSize); bs != nil {
			shadows = append(shadows, *bs)
		}
	}
	return shadows
}

// splitTopLevelCommas splits a string on commas that are not inside
// parentheses. This handles cases like "rgba(0,0,0,0.5) 2px 2px, 0 0 5px red".
func splitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// indexByteAtTopLevel returns the index of the first occurrence of b in s
// at paren depth 0, or -1 if b never appears at top level. Used to find
// CSS-grammar separators (e.g. the `/` between size and line-height in
// the font shorthand) without confusing them for characters inside
// functional values like calc(2em / 2).
func indexByteAtTopLevel(s string, b byte) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && c == b {
			return i
		}
	}
	return -1
}

// splitTopLevelFields splits a string on whitespace at paren depth 0,
// keeping functional values like "calc(50% - 8px)" or "min(10px, 5%)"
// intact as a single field. Differs from strings.Fields, which would
// split such values on their internal whitespace.
func splitTopLevelFields(s string) []string {
	var parts []string
	depth := 0
	start := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
			if start >= 0 {
				parts = append(parts, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		parts = append(parts, s[start:])
	}
	return parts
}
