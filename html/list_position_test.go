// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"
)

// list-style-position parsing, standalone and via the list-style shorthand.
func TestListStylePositionParsing(t *testing.T) {
	c := &converter{}
	cases := []struct {
		prop, val string
		wantPos   string
		wantType  string // "" = leave type unasserted
		checkType bool
	}{
		{"list-style-position", "inside", "inside", "", false},
		{"list-style-position", "INSIDE", "inside", "", false},
		{"list-style-position", "outside", "outside", "", false},
		{"list-style-position", "bogus", "", "", false}, // invalid ignored
		// Shorthand carries both type and position.
		{"list-style", "disc inside", "inside", "disc", true},
		{"list-style", "inside decimal", "inside", "decimal", true},
		// Shorthand with only a type leaves position at the default (outside).
		{"list-style", "decimal", "", "decimal", true},
		// A bare none still sets the type to none.
		{"list-style", "none", "", "none", true},
		// A marker image url() is not mistaken for the type keyword.
		{"list-style", "url(dot.png) square inside", "inside", "square", true},
	}
	for _, tc := range cases {
		var s computedStyle
		c.applyProperty(tc.prop, tc.val, &s)
		if s.ListStylePosition != tc.wantPos {
			t.Errorf("%s: %q → ListStylePosition=%q, want %q", tc.prop, tc.val, s.ListStylePosition, tc.wantPos)
		}
		if tc.checkType && s.ListStyleType != tc.wantType {
			t.Errorf("%s: %q → ListStyleType=%q, want %q", tc.prop, tc.val, s.ListStyleType, tc.wantType)
		}
	}
}

// End-to-end: list-style-position: inside renders the marker inline with the
// first content line and a wrapped continuation aligns under the marker. The
// stream must contain both the marker and the item text (no content dropped).
func TestListStylePositionInsideRenders(t *testing.T) {
	htmlStr := `<style>
		ol { list-style-position: inside; }
	</style>
	<ol><li>Wrapping clause body that is long enough to span two lines.</li></ol>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)
	if !strings.Contains(stream, "(1.) Tj") {
		t.Errorf("inside list: marker (1.) Tj missing\nstream:\n%s", stream)
	}
	// The first word renders as a kerned TJ array; assert a later literal word.
	if !strings.Contains(stream, "(clause) Tj") {
		t.Errorf("inside list: body word (clause) Tj missing\nstream:\n%s", stream)
	}
}
