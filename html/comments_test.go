// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestRemoveComments(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"between text", `<div>Net <!-- c --> 30</div>`, `<div>Net  30</div>`},
		{"two in a row", `<table><tr><td>a<!--x--><!--y-->b</td></tr></table>`, `<td>ab</td>`},
		{"between spans", `<p><span>a</span><!--
		--><span>b</span></p>`, `<p><span>a</span><span>b</span></p>`},
		{"nested", `<div><p>x<!-- c -->y</p></div>`, `<div><p>xy</p></div>`},
		{"at the edges", `<p><!-- a -->x<!-- b --></p>`, `<p>x</p>`},
		{"in style text kept", `<style>/* <!-- c --> */</style>`, `<style>/* <!-- c --> */</style>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(tt.in))
			if err != nil {
				t.Fatal(err)
			}
			removeComments(doc)
			var sb strings.Builder
			if err := html.Render(&sb, doc); err != nil {
				t.Fatal(err)
			}
			if got := sb.String(); !strings.Contains(got, tt.want) {
				t.Errorf("got %s, want it to contain %s", got, tt.want)
			}
			for n := range doc.Descendants() {
				if n.Type == html.TextNode && n.NextSibling != nil && n.NextSibling.Type == html.TextNode {
					t.Errorf("adjacent text nodes %q and %q", n.Data, n.NextSibling.Data)
				}
			}
		})
	}
}
