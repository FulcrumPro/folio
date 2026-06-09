// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// formatCounterValue: table-driven coverage of every style keyword,
// including none and the unknown fallback to decimal.
func TestFormatCounterValue(t *testing.T) {
	tests := []struct {
		n     int
		style string
		want  string
	}{
		{4, "", "4"},
		{4, "decimal", "4"},
		{4, "DECIMAL", "4"}, // case-insensitive
		{1, "decimal-leading-zero", "01"},
		{9, "decimal-leading-zero", "09"},
		{10, "decimal-leading-zero", "10"},
		{0, "decimal-leading-zero", "00"},
		{4, "lower-roman", "iv"},
		{4, "upper-roman", "IV"},
		{1, "lower-alpha", "a"},
		{2, "lower-latin", "b"},
		{1, "upper-alpha", "A"},
		{27, "upper-latin", "AA"},
		{5, "none", ""},
		{5, "  Upper-Roman  ", "V"}, // trimmed + lowercased
		{7, "bogus-style", "7"},     // unknown -> decimal fallback
	}
	for _, tt := range tests {
		if got := formatCounterValue(tt.n, tt.style); got != tt.want {
			t.Errorf("formatCounterValue(%d, %q) = %q, want %q", tt.n, tt.style, got, tt.want)
		}
	}
}

// counter() honors the optional list-style argument for regular counters.
func TestResolveContentValueCounterStyle(t *testing.T) {
	c := &converter{counters: make(map[string][]int)}
	c.resetCounter("c", 0)
	c.incrementCounter("c", 4) // c == 4

	cases := []struct {
		expr string
		want string
	}{
		{`counter(c, upper-roman)`, "IV"},
		{`counter(c, lower-roman)`, "iv"},
		{`counter(c, lower-alpha)`, "d"},
		{`counter(c, upper-alpha)`, "D"},
		{`counter(c, decimal-leading-zero)`, "04"},
		{`counter(c, decimal)`, "4"},
		{`counter(c)`, "4"}, // default unchanged
		{`counter(c, none)`, ""},
	}
	for _, tc := range cases {
		if got := c.resolveContentValue(tc.expr); got != tc.want {
			t.Errorf("resolveContentValue(%q) = %q, want %q", tc.expr, got, tc.want)
		}
	}
}

// counters() accepts an optional 3rd style argument, and without it the
// output is identical to the prior decimal behavior.
func TestResolveContentValueCountersStyle(t *testing.T) {
	c := &converter{counters: make(map[string][]int)}
	c.resetCounter("item", 0)
	c.incrementCounter("item", 1) // 1
	c.resetCounter("item", 0)     // nested
	c.incrementCounter("item", 2) // 2

	// With upper-alpha style: 1 -> A, 2 -> B.
	if got := c.resolveContentValue(`counters(item, ".", upper-alpha)`); got != "A.B" {
		t.Errorf("counters with upper-alpha = %q, want %q", got, "A.B")
	}
	// Custom separator with style.
	if got := c.resolveContentValue(`counters(item, " > ", lower-roman)`); got != "i > ii" {
		t.Errorf("counters with lower-roman = %q, want %q", got, "i > ii")
	}
	// No style: behavior identical to before (dotted decimals).
	if got := c.resolveContentValue(`counters(item, ".")`); got != "1.2" {
		t.Errorf("counters no style = %q, want %q", got, "1.2")
	}
	// Separator that itself contains a comma must be preserved intact
	// (top-level comma splitting ignores commas inside quotes).
	if got := c.resolveContentValue(`counters(item, ", ")`); got != "1, 2" {
		t.Errorf("counters comma separator = %q, want %q", got, "1, 2")
	}
	// Comma-containing separator together with a style argument.
	if got := c.resolveContentValue(`counters(item, ", ", upper-roman)`); got != "I, II" {
		t.Errorf("counters comma separator + style = %q, want %q", got, "I, II")
	}
}

// renderStreamText renders the elements through the full pipeline and returns
// the first page's content stream as a string. Standard (Helvetica) text is
// emitted as literal "(...) Tj" strings, so counter output is greppable.
func renderStreamText(t *testing.T, elems []layout.Element) string {
	t.Helper()
	r := layout.NewRenderer(612, 792, layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	for _, e := range elems {
		r.Add(e)
	}
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least one rendered page")
	}
	return string(pages[0].Stream.Bytes())
}

// List-walk wiring: li counter-increment must fire for each item during the
// list walk, and a nested ol's reset + nested li increment must produce
// correctly scoped counters() values. Each li has a <p> block child so it
// takes the element path, whose walkChildren resolves the p::before content.
func TestListWalkCounterIncrement(t *testing.T) {
	htmlStr := `<style>
		ol { counter-reset: item }
		li { counter-increment: item }
		li p::before { content: counters(item, ".") " " }
	</style>
	<ol><li><p>First</p><ol><li><p>Inner</p></li></ol></li><li><p>Second</p></li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	// The counters() prefix is rendered as its own Tj string immediately
	// before the paragraph text. "1" before First, "1.1" before Inner (nested
	// reset + nested increment), "2" before Second (outer increment fired).
	for _, want := range []string{"(1) Tj", "(1.1) Tj", "(2) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
	// Sanity: the body text is present so we know we read the right content.
	for _, want := range []string{"(First) Tj", "(Inner) Tj", "(Second) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing body text %q", want)
		}
	}
}

// A counter-reset declared on each <li> must be scoped per item: the push
// is popped after the item's subtree, so a sibling sees a fresh single-level
// reset rather than an accumulating stack. counters() (which joins the whole
// stack) makes an unbalanced pop observable: without the pop the second item
// would render "2:41-41" instead of "2:41".
func TestListItemCounterResetScoped(t *testing.T) {
	htmlStr := `<style>
		ol { counter-reset: item }
		li { counter-increment: item; counter-reset: note 41 }
		li p::before { content: counter(item) ":" counters(note, "-") " " }
	</style>
	<ol><li><p>A</p></li><li><p>B</p></li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)
	for _, want := range []string{"(1:41) Tj", "(2:41) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("rendered stream missing %q\nstream:\n%s", want, stream)
		}
	}
	if strings.Contains(stream, "41-41") {
		t.Errorf("counter-reset stack leaked across siblings (found 41-41)\nstream:\n%s", stream)
	}
}

// Regression: a plain ordered list with no counter CSS still renders the
// default decimal markers "1." / "2." unchanged by the counter wiring.
func TestPlainOrderedListMarkersUnchanged(t *testing.T) {
	elems, err := Convert(`<ol><li>a</li><li>b</li></ol>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)
	for _, want := range []string{"(1.) Tj", "(2.) Tj", "(a) Tj", "(b) Tj"} {
		if !strings.Contains(stream, want) {
			t.Errorf("plain ordered list missing %q\nstream:\n%s", want, stream)
		}
	}
}
