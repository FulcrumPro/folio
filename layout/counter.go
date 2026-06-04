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
//
// A `counter(page)` with an explicit list-style argument (e.g.
// `counter(page, upper-roman)`) is encoded as `{counter(page,STYLE)}`,
// where STYLE is the normalized list-style-type. The style-less form
// below is equivalent to `{counter(page,decimal)}` and is kept verbatim
// for backward compatibility with existing emitters.
const CounterPagePlaceholder = "{counter(page)}"

// CounterPagesPlaceholder is the deferred sentinel for CSS
// `counter(pages)`. It is substituted with the document's final page
// total once pagination is complete and emission begins. The styled
// form is `{counter(pages,STYLE)}` (see CounterPagePlaceholder).
const CounterPagesPlaceholder = "{counter(pages)}"

// CounterPlaceholder builds the placeholder token for a page/pages
// counter with an optional list-style. name must be "page" or "pages";
// style is a list-style-type ("" or "decimal" yields the canonical
// style-less placeholder). The token is opaque to everything between
// emission of the content string and the final substitution pass.
func CounterPlaceholder(name, style string) string {
	style = strings.TrimSpace(strings.ToLower(style))
	// Guard against a style argument that contains the placeholder's own
	// delimiters (N-a). A stray '}' or ')' would corrupt or prematurely
	// terminate the {counter(...)} token at substitution time, leaking the
	// remainder as literal text. Such input is never a valid CSS
	// list-style-type, so drop the style and fall back to decimal.
	if strings.ContainsAny(style, "})") {
		style = ""
	}
	if style == "" || style == "decimal" {
		return "{counter(" + name + ")}"
	}
	return "{counter(" + name + "," + style + ")}"
}

// counterMeasureDigits is the digit string used to reserve width for a
// counter placeholder during paragraph measurement. The reserved width
// must be at least as wide as the worst-case substituted value so that
// line breaks decided at layout time remain valid after substitution.
// Four "8" digits cover documents up to 9,999 pages; "8" is chosen as a
// conservative wide-glyph approximation. Documents exceeding this bound
// still render correctly (substitution writes the real digits) but may
// see counter values overflow their reserved trailing whitespace.
const counterMeasureDigits = "8888"

// formatCounter renders the integer n using the given CSS list-style-type.
// Supported styles: decimal (default), decimal-leading-zero, lower-roman,
// upper-roman, lower-alpha, upper-alpha. Unknown styles fall back to
// decimal. n is the 1-based counter value.
func formatCounter(n int, style string) string {
	switch strings.TrimSpace(strings.ToLower(style)) {
	case "decimal-leading-zero":
		s := strconv.Itoa(n)
		if n >= 0 && n < 10 {
			return "0" + s
		}
		return s
	case "lower-roman":
		return strings.ToLower(romanNumeral(n))
	case "upper-roman":
		return romanNumeral(n)
	case "lower-alpha", "lower-latin":
		return alphaNumeral(n, 'a')
	case "upper-alpha", "upper-latin":
		return alphaNumeral(n, 'A')
	default: // "decimal", "" and any unknown style
		return strconv.Itoa(n)
	}
}

// romanNumeral converts a positive integer to an upper-case Roman
// numeral. Values <= 0 fall back to the decimal representation (Roman
// numerals have no zero or negative form).
func romanNumeral(n int) string {
	if n <= 0 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	return b.String()
}

// alphaNumeral converts a positive integer to a bijective base-26
// alphabetic numeral (1→a, 26→z, 27→aa, ...) starting from base ('a' or
// 'A'). Values <= 0 fall back to the decimal representation.
func alphaNumeral(n int, base rune) string {
	if n <= 0 {
		return strconv.Itoa(n)
	}
	var letters []byte
	for n > 0 {
		n--
		letters = append(letters, byte(base)+byte(n%26))
		n /= 26
	}
	// Reverse (most-significant digit first).
	for i, j := 0, len(letters)-1; i < j; i, j = i+1, j-1 {
		letters[i], letters[j] = letters[j], letters[i]
	}
	return string(letters)
}

// substituteCounter replaces every page/pages counter placeholder in s —
// including styled variants like {counter(page,upper-roman)} — with the
// value produced by valueFor(name, style), where name is "page" or
// "pages". Returns s unchanged when no placeholder is present. Tokens
// whose counter name is not page/pages (e.g. {counter(chapter)}) are
// left untouched.
func substituteCounter(s string, valueFor func(name, style string) string) string {
	if !strings.Contains(s, "{counter(") {
		return s
	}
	var b strings.Builder
	for {
		idx := strings.Index(s, "{counter(")
		if idx < 0 {
			b.WriteString(s)
			break
		}
		close := strings.IndexByte(s[idx:], '}')
		if close < 0 {
			b.WriteString(s)
			break
		}
		close += idx
		// inner = the text inside {counter(...)} between the "(" and "}".
		// e.g. "page)" or "page,upper-roman)".
		inner := s[idx+len("{counter(") : close]
		inner = strings.TrimSuffix(strings.TrimSpace(inner), ")")
		name, style := inner, ""
		if comma := strings.IndexByte(inner, ','); comma >= 0 {
			name = strings.TrimSpace(inner[:comma])
			style = strings.TrimSpace(inner[comma+1:])
		}
		name = strings.ToLower(name)
		if name == "page" || name == "pages" {
			b.WriteString(s[:idx])
			b.WriteString(valueFor(name, style))
			s = s[close+1:]
			continue
		}
		// Unknown counter name: emit verbatim up to and including '}' and
		// continue scanning the remainder.
		b.WriteString(s[:close+1])
		s = s[close+1:]
	}
	return b.String()
}

// substituteCounters replaces page-counter placeholders in s with their
// resolved 1-based page index and total page count, honoring any
// list-style argument carried by the placeholder. Returns s unchanged
// when no placeholder is present.
func substituteCounters(s string, pageIdx, totalPages int) string {
	return substituteCounter(s, func(name, style string) string {
		if name == "pages" {
			return formatCounter(totalPages, style)
		}
		return formatCounter(pageIdx+1, style)
	})
}

// expandCountersForMeasure replaces page-counter placeholders with a
// fixed-width digit reservation string used only for measurement. The
// returned text is used to compute Word.Width so that line breaks are
// stable across digit-count transitions (page 9 → 10, 99 → 100). The
// real placeholder remains on Word.Text and is substituted at emit
// time.
func expandCountersForMeasure(s string) string {
	return substituteCounter(s, func(name, style string) string {
		return counterMeasureDigits
	})
}

// hasCounterPlaceholder reports whether s contains a page or pages
// counter placeholder needing emit-time substitution. Counter tokens
// for other names (e.g. {counter(chapter)}) are not flagged.
func hasCounterPlaceholder(s string) bool {
	if !strings.Contains(s, "{counter(") {
		return false
	}
	found := false
	substituteCounter(s, func(name, style string) string {
		found = true
		return ""
	})
	return found
}
