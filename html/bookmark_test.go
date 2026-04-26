// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"testing"

	gohtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TestResolveBookmarkLabelDefaults(t *testing.T) {
	cases := []struct {
		raw, text, want string
	}{
		{"", "Section text", "Section text"},
		{"content()", "Section text", "Section text"},
		{`"Custom"`, "Section text", "Custom"},
		{`'Custom'`, "Section text", "Custom"},
		{"  content()  ", "Section text", "Section text"},
		{`  "Trimmed"  `, "Section text", "Trimmed"},
	}
	for _, tc := range cases {
		if got := resolveBookmarkLabel(tc.raw, nil, tc.text); got != tc.want {
			t.Errorf("resolveBookmarkLabel(%q, _, %q) = %q, want %q", tc.raw, tc.text, got, tc.want)
		}
	}
}

func TestResolveBookmarkLabelAttr(t *testing.T) {
	n := &gohtml.Node{
		Type:     gohtml.ElementNode,
		Data:     "figure",
		DataAtom: atom.Figure,
		Attr: []gohtml.Attribute{
			{Key: "data-caption", Val: "Figure 7"},
			{Key: "class", Val: "highlight"},
		},
	}
	if got := resolveBookmarkLabel("attr(data-caption)", n, "fallback"); got != "Figure 7" {
		t.Errorf("attr(data-caption): got %q, want %q", got, "Figure 7")
	}
	// Missing attribute falls back to element text.
	if got := resolveBookmarkLabel("attr(missing)", n, "fallback"); got != "fallback" {
		t.Errorf("attr(missing): got %q, want %q", got, "fallback")
	}
	// Empty attr() name falls back to element text.
	if got := resolveBookmarkLabel("attr()", n, "fallback"); got != "fallback" {
		t.Errorf("attr(): got %q, want %q", got, "fallback")
	}
}

// TestApplyBookmarkLevelParser pins the parser semantics for
// bookmark-level. CSS GCPM accepts a positive integer or the keyword
// "none"; 0, negatives, and out-of-range values must be rejected so
// downstream layers (BookmarkAnchor wrap, Heading override) can rely on
// BookmarkLevel ∈ {-1, 1..6} whenever BookmarkLevelSet is true.
func TestApplyBookmarkLevelParser(t *testing.T) {
	cases := []struct {
		val     string
		wantSet bool
		wantLvl int
	}{
		{"none", true, -1},
		{"NONE", true, -1},
		{"1", true, 1},
		{"6", true, 6},
		{"0", false, 0}, // invalid: spec requires >= 1 or none
		{"-1", false, 0},
		{"7", false, 0}, // out of Folio's H1-H6 range
		{"abc", false, 0},
		{"", false, 0},
	}
	c := &converter{}
	for _, tc := range cases {
		var s computedStyle
		c.applyProperty("bookmark-level", tc.val, &s)
		if s.BookmarkLevelSet != tc.wantSet {
			t.Errorf("bookmark-level: %q → BookmarkLevelSet=%v, want %v",
				tc.val, s.BookmarkLevelSet, tc.wantSet)
		}
		if s.BookmarkLevel != tc.wantLvl {
			t.Errorf("bookmark-level: %q → BookmarkLevel=%d, want %d",
				tc.val, s.BookmarkLevel, tc.wantLvl)
		}
	}
}

func TestIsHeadingNode(t *testing.T) {
	for _, a := range []atom.Atom{atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6} {
		n := &gohtml.Node{Type: gohtml.ElementNode, DataAtom: a}
		if !isHeadingNode(n) {
			t.Errorf("expected %v to be a heading node", a)
		}
	}
	for _, a := range []atom.Atom{atom.P, atom.Div, atom.Figure, atom.Span} {
		n := &gohtml.Node{Type: gohtml.ElementNode, DataAtom: a}
		if isHeadingNode(n) {
			t.Errorf("did not expect %v to be a heading node", a)
		}
	}
	if isHeadingNode(nil) {
		t.Error("nil should not be a heading node")
	}
}
