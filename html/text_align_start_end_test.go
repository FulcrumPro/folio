// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestParseTextAlignStartEnd covers the new direction-relative
// keywords. Pre-fix parseTextAlign returned (AlignLeft, false) for
// any value outside left/center/right/justify, so `start` and `end`
// silently fell back to the inherited TextAlign (default AlignLeft).
// Post-fix the parser returns the LTR-correct best guess plus a
// keyword string for late binding.
func TestParseTextAlignStartEnd(t *testing.T) {
	tests := []struct {
		value       string
		wantAlign   layout.Align
		wantKeyword string
		wantOk      bool
	}{
		{"left", layout.AlignLeft, "", true},
		{"center", layout.AlignCenter, "", true},
		{"right", layout.AlignRight, "", true},
		{"justify", layout.AlignJustify, "", true},
		{"start", layout.AlignLeft, "start", true},
		{"end", layout.AlignRight, "end", true},
		{"START", layout.AlignLeft, "start", true},
		{"  end  ", layout.AlignRight, "end", true},
		{"match-parent", layout.AlignLeft, "", false}, // not yet supported
		{"justify-all", layout.AlignLeft, "", false},  // not yet supported
		{"", layout.AlignLeft, "", false},
		{"banana", layout.AlignLeft, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			align, kw, ok := parseTextAlign(tc.value)
			if ok != tc.wantOk {
				t.Errorf("parseTextAlign(%q) ok = %v, want %v", tc.value, ok, tc.wantOk)
			}
			if align != tc.wantAlign {
				t.Errorf("parseTextAlign(%q) align = %v, want %v", tc.value, align, tc.wantAlign)
			}
			if kw != tc.wantKeyword {
				t.Errorf("parseTextAlign(%q) keyword = %q, want %q", tc.value, kw, tc.wantKeyword)
			}
		})
	}
}

// TestResolveTextAlignDirectionMatrix is the matrix that locks in the
// late binding behaviour: the resolver should map start/end to
// left/right based on style.Direction, while leaving non-direction-
// relative keywords untouched.
func TestResolveTextAlignDirectionMatrix(t *testing.T) {
	tests := []struct {
		name      string
		keyword   string
		fallback  layout.Align
		direction layout.Direction
		want      layout.Align
	}{
		// Direction-relative under LTR / Auto.
		{"start under LTR", "start", layout.AlignLeft, layout.DirectionLTR, layout.AlignLeft},
		{"start under Auto", "start", layout.AlignLeft, layout.DirectionAuto, layout.AlignLeft},
		{"end under LTR", "end", layout.AlignRight, layout.DirectionLTR, layout.AlignRight},
		{"end under Auto", "end", layout.AlignRight, layout.DirectionAuto, layout.AlignRight},
		// Direction-relative under RTL — the issue this fix closes.
		{"start under RTL", "start", layout.AlignLeft, layout.DirectionRTL, layout.AlignRight},
		{"end under RTL", "end", layout.AlignRight, layout.DirectionRTL, layout.AlignLeft},
		// Non-direction-relative keywords pass through regardless of direction.
		{"left under RTL", "", layout.AlignLeft, layout.DirectionRTL, layout.AlignLeft},
		{"right under RTL", "", layout.AlignRight, layout.DirectionRTL, layout.AlignRight},
		{"center under RTL", "", layout.AlignCenter, layout.DirectionRTL, layout.AlignCenter},
		{"justify under RTL", "", layout.AlignJustify, layout.DirectionRTL, layout.AlignJustify},
		// Empty fallback (unset TextAlign) under direction-relative keyword.
		{"unknown keyword under RTL falls back", "match-parent", layout.AlignLeft, layout.DirectionRTL, layout.AlignLeft},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			style := computedStyle{
				TextAlign:        tc.fallback,
				TextAlignKeyword: tc.keyword,
				Direction:        tc.direction,
			}
			got := resolveTextAlign(style)
			if got != tc.want {
				t.Errorf("resolveTextAlign(keyword=%q, fallback=%v, dir=%v) = %v, want %v",
					tc.keyword, tc.fallback, tc.direction, got, tc.want)
			}
		})
	}
}

// TestResolveTextAlignLastDirectionMatrix mirrors the above for
// text-align-last (which has the same direction-relative keywords).
func TestResolveTextAlignLastDirectionMatrix(t *testing.T) {
	tests := []struct {
		keyword   string
		fallback  layout.Align
		direction layout.Direction
		want      layout.Align
	}{
		{"start", layout.AlignLeft, layout.DirectionRTL, layout.AlignRight},
		{"end", layout.AlignRight, layout.DirectionRTL, layout.AlignLeft},
		{"start", layout.AlignLeft, layout.DirectionLTR, layout.AlignLeft},
		{"", layout.AlignJustify, layout.DirectionRTL, layout.AlignJustify},
	}
	for _, tc := range tests {
		name := tc.keyword
		switch tc.direction {
		case layout.DirectionRTL:
			name += "/RTL"
		case layout.DirectionLTR:
			name += "/LTR"
		default:
			name += "/Auto"
		}
		t.Run(name, func(t *testing.T) {
			style := computedStyle{
				TextAlignLast:        tc.fallback,
				TextAlignLastKeyword: tc.keyword,
				Direction:            tc.direction,
			}
			got := resolveTextAlignLast(style)
			if got != tc.want {
				t.Errorf("resolveTextAlignLast = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestTextAlignStartEndDeclarationOrderIndependence verifies the
// late-binding contract: the resolved alignment is correct regardless
// of whether `direction` is declared before or after `text-align: start`
// in the same block. CSS declarations are processed in source order,
// so an early-binding implementation would get the wrong answer when
// `text-align: start` is declared before `direction: rtl`.
func TestTextAlignStartEndDeclarationOrderIndependence(t *testing.T) {
	tests := []struct {
		name string
		css  string
		want layout.Align
	}{
		{
			name: "text-align before direction",
			css:  "p { text-align: start; direction: rtl; }",
			want: layout.AlignRight,
		},
		{
			name: "direction before text-align",
			css:  "p { direction: rtl; text-align: start; }",
			want: layout.AlignRight,
		},
		{
			name: "LTR text-align: end",
			css:  "p { text-align: end; direction: ltr; }",
			want: layout.AlignRight,
		},
		{
			name: "RTL text-align: end",
			css:  "p { text-align: end; direction: rtl; }",
			want: layout.AlignLeft,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html := "<html><head><style>" + tc.css + "</style></head><body><p>x</p></body></html>"
			elems, err := Convert(html, nil)
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if len(elems) != 1 {
				t.Fatalf("expected 1 element, got %d", len(elems))
			}
			p, ok := elems[0].(*layout.Paragraph)
			if !ok {
				t.Fatalf("expected *layout.Paragraph, got %T", elems[0])
			}
			if p.Align() != tc.want {
				t.Errorf("paragraph align = %v, want %v", p.Align(), tc.want)
			}
		})
	}
}

// Note: there's a pre-existing inheritance gap that's out of scope
// for this PR — `style.TextAlignSet` is NOT carried by inherit(),
// so a parent's `text-align: start` doesn't reach a child paragraph
// through the `if style.TextAlignSet` consumer guards. The fix would
// be to inherit TextAlignSet (and the symmetric TextAlignLastSet),
// but that affects every text-align-on-parent case, not just the
// new direction-relative keywords. Filed separately. The cascade
// still works when the property is declared directly on the
// rendered element, which is the common case.

// Behaviour change for <th> default-center logic:
// Pre-fix, the gate in converter_table.go was a raw
// `cellStyle.TextAlign == AlignLeft → re-center`. So an author
// who wrote `th { text-align: left }` got their left explicitly
// overridden to center because the resolved TextAlign matched the
// re-center sentinel. Post-fix, the gate is
// `!cellStyle.TextAlignSet && resolveTextAlign(cellStyle) == AlignLeft`,
// which preserves explicit author choice (left or start/end
// resolving to left under direction). Default <th> still centers.
//
// Cell-level alignment is internal to layout.Cell (no public Align()
// getter), so this is locked in via the converter_table.go code
// comment and the resolveTextAlign matrix above. A render-level
// regression would require adding test-only accessors to layout.Cell
// — out of scope here.

// TestTextAlignNonRelativeUnaffected guards against regression in the
// common path: `text-align: center` with `direction: rtl` should
// still center, not flip. Same for left/right/justify.
func TestTextAlignNonRelativeUnaffected(t *testing.T) {
	cases := []struct {
		decl string
		want layout.Align
	}{
		{"text-align: left;  direction: rtl;", layout.AlignLeft},
		{"text-align: right; direction: rtl;", layout.AlignRight},
		{"text-align: center; direction: rtl;", layout.AlignCenter},
		{"text-align: justify; direction: rtl;", layout.AlignJustify},
	}
	for _, tc := range cases {
		t.Run(strings.SplitN(tc.decl, ";", 2)[0], func(t *testing.T) {
			html := "<html><head><style>p {" + tc.decl + "}</style></head><body><p>x</p></body></html>"
			elems, err := Convert(html, nil)
			if err != nil {
				t.Fatal(err)
			}
			p := elems[0].(*layout.Paragraph)
			if p.Align() != tc.want {
				t.Errorf("align = %v, want %v", p.Align(), tc.want)
			}
		})
	}
}
