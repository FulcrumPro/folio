// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// isHeadingNode reports whether n is one of <h1>..<h6>. Headings own
// their bookmark metadata (see convertHeading); non-headings opt in via
// the BookmarkAnchor wrapper applied in convertElement.
func isHeadingNode(n *html.Node) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		return true
	}
	return false
}

// resolveBookmarkLabel turns a CSS bookmark-label expression into the
// concrete string used in the PDF outline. Per CSS GCPM:
//
//   - empty / unset             → use elementText (the default)
//   - content()                 → use elementText
//   - attr(NAME)                → use the value of the NAME attribute
//   - "Literal" / 'Literal'     → use the literal string (quotes stripped)
//
// An attr() reference to a missing attribute falls back to elementText,
// matching CSS attr() semantics for missing attributes (the property
// uses its initial value rather than emitting a debug placeholder).
func resolveBookmarkLabel(raw string, n *html.Node, elementText string) string {
	v := strings.TrimSpace(raw)
	if v == "" || v == "content()" {
		return elementText
	}
	if strings.HasPrefix(v, "attr(") && strings.HasSuffix(v, ")") {
		name := strings.TrimSpace(v[len("attr(") : len(v)-1])
		if name == "" {
			return elementText
		}
		if val := getAttr(n, name); val != "" {
			return val
		}
		return elementText
	}
	return strings.Trim(v, `"'`)
}
