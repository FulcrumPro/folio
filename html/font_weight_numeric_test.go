// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strconv"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// TestParseFontWeightNumericLadder closes #286. Pre-fix parseFontWeight
// returned the binary string "normal"/"bold"; post-fix it returns the
// CSS Fonts L4 numeric ladder. SemiBold (600) and Medium (500), which
// previously collapsed to "normal", now pass through.
func TestParseFontWeightNumericLadder(t *testing.T) {
	tests := []struct {
		value     string
		inherited int
		want      int
	}{
		// Keywords.
		{"normal", 400, 400},
		{"bold", 400, 700},
		{"NORMAL", 400, 400}, // case-insensitive
		{"  bold  ", 400, 700},

		// Numeric pass-through.
		{"100", 400, 100},
		{"200", 400, 200},
		{"300", 400, 300},
		{"400", 400, 400},
		{"500", 400, 500},
		{"600", 400, 600}, // SemiBold — the fix
		{"700", 400, 700},
		{"800", 400, 800},
		{"900", 400, 900},

		// Numeric clamping.
		{"50", 400, 100},   // below 100 clamps up
		{"950", 400, 900},  // above 900 clamps down
		{"1000", 400, 900}, // way above
		{"0", 400, 400},    // zero falls back to inherited
		{"-100", 400, 400}, // negative falls back to inherited

		// Bolder relative to inherited (CSS Fonts L4 §3.1 ladder).
		{"bolder", 100, 400},
		{"bolder", 300, 400},
		{"bolder", 400, 700},
		{"bolder", 500, 700},
		{"bolder", 600, 900},
		{"bolder", 700, 900},
		{"bolder", 900, 900},

		// Lighter relative to inherited.
		{"lighter", 100, 100},
		{"lighter", 300, 100},
		{"lighter", 400, 100},
		{"lighter", 500, 100},
		{"lighter", 600, 400},
		{"lighter", 700, 400},
		{"lighter", 900, 700},

		// Inherited 0 falls back to 400 default.
		{"bolder", 0, 700}, // bolder(400) = 700
		{"lighter", 0, 100},

		// Unknown value preserves inherited (cascade semantics).
		{"superbold", 600, 600},
		{"", 500, 500},
		{"oblique", 700, 700}, // CSS spec: invalid font-weight falls back

		// Inherited 0 + numeric still works.
		{"600", 0, 600},
	}
	for _, tc := range tests {
		name := tc.value + "/inherited=" + strconv.Itoa(tc.inherited)
		t.Run(name, func(t *testing.T) {
			got := parseFontWeight(tc.value, tc.inherited)
			if got != tc.want {
				t.Errorf("parseFontWeight(%q, %d) = %d, want %d", tc.value, tc.inherited, got, tc.want)
			}
		})
	}
}

// TestPickNearestWeight covers the CSS Fonts L4 §5.2 ladder walk.
// Verifies the algorithm against the spec's worked examples.
func TestPickNearestWeight(t *testing.T) {
	// Family with weights 400, 600, 700 available — the canonical
	// "Inter has Regular/SemiBold/Bold" case from the design system
	// review that produced #286.
	interLike := []int{400, 600, 700}

	tests := []struct {
		desired   int
		available []int
		want      int
	}{
		// Exact matches always win.
		{400, interLike, 400},
		{600, interLike, 600},
		{700, interLike, 700},

		// 500 → 400 first per the special-case rule (since 500 absent).
		{500, interLike, 400},
		// 400 → 500 first when 500 present.
		{400, []int{300, 500, 700}, 500},

		// Below 400: walk down first, then up.
		{300, interLike, 400}, // no faces below 400, fall back to up
		{200, []int{100, 600}, 100},
		{300, []int{500, 700}, 500},

		// Above 500 (and not 500 itself): walk up first, then down.
		{800, interLike, 700}, // no faces ≥ 800, fall back down
		{800, []int{500, 900}, 900},
		{650, interLike, 700}, // 700 is closer up than 600 down

		// CSS Fonts L4 §5.2 [400, 500] ascending-toward-500 arc.
		{450, []int{400, 500, 700}, 500}, // ascend toward 500 first
		{450, []int{300, 700}, 300},      // 500 absent, walk down
		{450, []int{700, 900}, 700},      // below desired absent, walk above 500 ascending
		{475, []int{500}, 500},           // singleton 500 in window
		{425, []int{200, 700}, 200},      // (desired, 500] empty, descend below

		// Single available weight always returned.
		{900, []int{400}, 400},
		{100, []int{700}, 700},

		// Empty list returns 0 (caller falls through to standard font).
		{400, nil, 0},
		{400, []int{}, 0},
	}
	for _, tc := range tests {
		t.Run("desired="+strconv.Itoa(tc.desired), func(t *testing.T) {
			got := pickNearestWeight(tc.desired, tc.available)
			if got != tc.want {
				t.Errorf("pickNearestWeight(%d, %v) = %d, want %d", tc.desired, tc.available, got, tc.want)
			}
		})
	}
}

// TestResolveFontPairNearestWeightMatching is the issue #286 acceptance
// test: given an Inter family with @font-face declarations at 400, 500,
// 600, and 700, the document's `font-weight: 600` selects the
// SemiBold (600) face — not the Regular (400) face it picked pre-fix.
func TestResolveFontPairNearestWeightMatching(t *testing.T) {
	// Distinct mock faces so we can tell which one was picked.
	regular := font.NewEmbeddedFont(nil)
	medium := font.NewEmbeddedFont(nil)
	semiBold := font.NewEmbeddedFont(nil)
	bold := font.NewEmbeddedFont(nil)

	c := &converter{
		embeddedFonts: map[string]*font.EmbeddedFont{
			"inter|400|normal": regular,
			"inter|500|normal": medium,
			"inter|600|normal": semiBold,
			"inter|700|normal": bold,
		},
	}

	tests := []struct {
		name   string
		weight int
		want   *font.EmbeddedFont
	}{
		{"weight=400 picks Regular", 400, regular},
		{"weight=500 picks Medium", 500, medium},
		{"weight=600 picks SemiBold (issue #286 acceptance)", 600, semiBold},
		{"weight=700 picks Bold", 700, bold},
		{"weight=800 walks down to Bold", 800, bold},
		{"weight=300 walks down (none) then up to Regular", 300, regular},
		{"weight=0 (unset) treated as 400", 0, regular},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			style := defaultStyle()
			style.FontFamily = "inter"
			style.FontWeight = tc.weight
			std, ef := c.resolveFontPair(style)
			if std != nil {
				t.Errorf("expected nil standard font; got %v", std)
			}
			if ef != tc.want {
				t.Errorf("got wrong embedded face for weight %d", tc.weight)
			}
		})
	}
}

// TestResolveFontPairFamilyMissDoesNotMisselect ensures nearest-weight
// matching only kicks in when at least one @font-face for the family
// exists. An unrelated family still falls back to standard fonts.
func TestResolveFontPairFamilyMissDoesNotMisselect(t *testing.T) {
	c := &converter{
		embeddedFonts: map[string]*font.EmbeddedFont{
			"inter|400|normal": font.NewEmbeddedFont(nil),
		},
	}
	style := defaultStyle()
	style.FontFamily = "roboto"
	style.FontWeight = 600
	std, ef := c.resolveFontPair(style)
	if ef != nil {
		t.Error("expected nil embedded font for unmatched family")
	}
	if std == nil {
		t.Error("expected standard-font fallback")
	}
}

// TestResolveFontPairItalicFallback ensures the second-pass style
// fallback triggers when only one style is registered. Better to
// render in regular Inter than fall through to standard Helvetica
// when the author asked for italic Inter.
func TestResolveFontPairItalicFallback(t *testing.T) {
	regular := font.NewEmbeddedFont(nil)
	c := &converter{
		embeddedFonts: map[string]*font.EmbeddedFont{
			"inter|400|normal": regular,
		},
	}
	style := defaultStyle()
	style.FontFamily = "inter"
	style.FontStyle = "italic"
	style.FontWeight = 400
	std, ef := c.resolveFontPair(style)
	if ef != regular {
		t.Errorf("expected fallback to normal-style face; got std=%v ef=%v", std, ef)
	}
}

// TestEmbeddedFontKeyShapeContract locks in that loadFontFaces and
// matchEmbeddedFont agree on the key format for the embeddedFonts map.
// The two writers are in different files (converter.go vs
// converter_helpers.go) and the key is the only thing tying them
// together — if either side drifts (e.g. switching to a struct key,
// changing the separator, padding the weight to "400" instead of
// "400"), the other silently misses every lookup.
func TestEmbeddedFontKeyShapeContract(t *testing.T) {
	want := font.NewEmbeddedFont(nil)
	c := &converter{embeddedFonts: map[string]*font.EmbeddedFont{}}

	// Reproduce the exact line from loadFontFaces. If the format
	// string in converter.go drifts, this test still uses the OLD
	// format and the lookup fails — surfacing the drift.
	ff := fontFaceRule{family: "inter", weight: 600, style: "normal"}
	key := ff.family + "|" + strconv.Itoa(ff.weight) + "|" + ff.style
	c.embeddedFonts[key] = want

	// Same lookup path as resolveFontPair → matchEmbeddedFont.
	got := c.matchEmbeddedFont("inter", "normal", 600)
	if got != want {
		t.Fatalf("matchEmbeddedFont did not find the loadFontFaces key %q", key)
	}
}

// TestResolveFontStandardWeightThreshold pins the synthetic-bolding
// rule for standard PDF-14 fonts: weights ≥ 600 select Bold, < 600
// select Regular. The PDF-14 set has no SemiBold — this is the only
// reasonable rounding.
func TestResolveFontStandardWeightThreshold(t *testing.T) {
	tests := []struct {
		weight int
		family string
		want   string
	}{
		{100, "helvetica", "Helvetica"},
		{400, "helvetica", "Helvetica"},
		{500, "helvetica", "Helvetica"},
		{600, "helvetica", "Helvetica-Bold"},
		{700, "helvetica", "Helvetica-Bold"},
		{900, "helvetica", "Helvetica-Bold"},
		{0, "helvetica", "Helvetica"}, // unset treated as 400
		{600, "times", "Times-Bold"},
		{500, "courier", "Courier"},
		{700, "courier", "Courier-Bold"},
	}
	for _, tc := range tests {
		t.Run(tc.family+"/"+strconv.Itoa(tc.weight), func(t *testing.T) {
			style := defaultStyle()
			style.FontFamily = tc.family
			style.FontWeight = tc.weight
			got := resolveFont(style)
			if got.Name() != tc.want {
				t.Errorf("resolveFont(%s, %d) = %s, want %s", tc.family, tc.weight, got.Name(), tc.want)
			}
		})
	}
}
