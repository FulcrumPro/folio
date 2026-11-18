// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import "golang.org/x/net/html"

// removeComments deletes every comment node under n and joins the text
// nodes on either side of each one. A browser lays out "Net <!-- c --> 30"
// as one line; left in the tree, the comment would end the inline run and
// put "30" on a line of its own.
func removeComments(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type != html.CommentNode {
			removeComments(c)
			c = next
			continue
		}
		prev := c.PrevSibling
		n.RemoveChild(c)
		if prev != nil && next != nil && prev.Type == html.TextNode && next.Type == html.TextNode {
			prev.Data += next.Data
			after := next.NextSibling
			n.RemoveChild(next)
			next = after
		}
		c = next
	}
}
