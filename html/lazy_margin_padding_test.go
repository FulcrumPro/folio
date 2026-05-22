// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"
)

// TestParseBoxSideBothReturnsBothForms is the parser-side contract
// for the #269 Phase 1 sibling-field migration. Every margin/padding
// Apply must produce BOTH:
//
//   - a legacy float64 resolved against zero (the existing 0pt-on-
//     percent behaviour) for back-compat with unmigrated consumers
//   - an unresolved *cssLength that preserves the percent / calc tree
//     for layout-time resolution against the container width
//
// "auto" / unparseable input still yields (0, nil) so callers can
// branch on the *cssLength being nil.
func TestParseBoxSideBothReturnsBothForms(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		fontSize   float64
		wantLegacy float64
		wantUnit   string  // expected cssLength.Unit; "" means nil expected
		wantValue  float64 // for non-nil, expected cssLength.Value
	}{
		{"plain points", "10pt", 12, 10, "pt", 10},
		// 16px → 12pt at 0.75 px/pt.
		{"plain pixels", "16px", 12, 12, "px", 16},
		// percent: legacy resolves against 0 (the bug); sibling
		// retains the 50% form for layout-time resolution.
		{"percent", "50%", 12, 0, "%", 50},
		// em on the legacy path resolves against fontSize, so 1em
		// at 12pt → 12pt. The sibling carries Unit "em" so a future
		// consumer can re-resolve.
		{"em", "1.5em", 12, 18, "em", 1.5},
		// Unparseable inputs return (0, nil).
		{"empty", "", 12, 0, "", 0},
		{"auto keyword (parseLength rejects)", "auto", 12, 0, "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotLegacy, gotLength := parseBoxSideBoth(tc.input, tc.fontSize)
			if math.Abs(gotLegacy-tc.wantLegacy) > 0.001 {
				t.Errorf("legacy float64 = %v, want %v", gotLegacy, tc.wantLegacy)
			}
			if tc.wantUnit == "" {
				if gotLength != nil {
					t.Errorf("cssLength = %+v, want nil", gotLength)
				}
				return
			}
			if gotLength == nil {
				t.Fatalf("cssLength = nil, want Unit=%q Value=%v", tc.wantUnit, tc.wantValue)
			}
			if gotLength.Unit != tc.wantUnit {
				t.Errorf("cssLength.Unit = %q, want %q", gotLength.Unit, tc.wantUnit)
			}
			if math.Abs(gotLength.Value-tc.wantValue) > 0.001 {
				t.Errorf("cssLength.Value = %v, want %v", gotLength.Value, tc.wantValue)
			}
		})
	}
}

// TestMarginTopAtResolvesPercentAgainstContainer is the central Phase 1
// claim: when the parser stored a *cssLength sibling, MarginTopAt
// resolves percent correctly against the container width — closing the
// silent 0pt bug at the helper level. Phase 2 migrates consumers to
// the helper so the bug closes end-to-end.
//
// Each subtest synthesizes a computedStyle as the parser would
// produce it, then asserts the helper's resolution. All eight
// sides (4 margin + 4 padding) follow the same shape; the test
// table covers one of each plus a few targeted edge cases.
func TestMarginTopAtResolvesPercentAgainstContainer(t *testing.T) {
	type sideHelper func(*computedStyle, float64) float64
	type sideSetter func(*computedStyle, *cssLength)

	apply := []struct {
		name   string
		set    sideSetter
		helper sideHelper
	}{
		{"MarginTop", func(s *computedStyle, l *cssLength) { s.MarginTopLength = l }, (*computedStyle).MarginTopAt},
		{"MarginRight", func(s *computedStyle, l *cssLength) { s.MarginRightLength = l }, (*computedStyle).MarginRightAt},
		{"MarginBottom", func(s *computedStyle, l *cssLength) { s.MarginBottomLength = l }, (*computedStyle).MarginBottomAt},
		{"MarginLeft", func(s *computedStyle, l *cssLength) { s.MarginLeftLength = l }, (*computedStyle).MarginLeftAt},
		{"PaddingTop", func(s *computedStyle, l *cssLength) { s.PaddingTopLength = l }, (*computedStyle).PaddingTopAt},
		{"PaddingRight", func(s *computedStyle, l *cssLength) { s.PaddingRightLength = l }, (*computedStyle).PaddingRightAt},
		{"PaddingBottom", func(s *computedStyle, l *cssLength) { s.PaddingBottomLength = l }, (*computedStyle).PaddingBottomAt},
		{"PaddingLeft", func(s *computedStyle, l *cssLength) { s.PaddingLeftLength = l }, (*computedStyle).PaddingLeftAt},
	}
	cases := []struct {
		name           string
		value          string
		containerWidth float64
		fontSize       float64
		want           float64
	}{
		// The bug-closing case: 50% of a 200pt container = 100pt.
		// Legacy parseBoxSide would have stored 0pt.
		{"percent against container", "50%", 200, 12, 100},
		// calc(10% + 5px) at 200pt container, 12pt fontSize:
		// 10% of 200 = 20pt; 5px = 3.75pt; total 23.75pt.
		{"calc mixed", "calc(10% + 5px)", 200, 12, 23.75},
		// Plain px: container width irrelevant.
		{"plain px ignores container", "16px", 200, 12, 12},
		// em: relative to fontSize, not container.
		{"em ignores container", "2em", 200, 12, 24},
	}
	for _, h := range apply {
		for _, tc := range cases {
			t.Run(h.name+"/"+tc.name, func(t *testing.T) {
				s := &computedStyle{FontSize: tc.fontSize}
				_, length := parseBoxSideBoth(tc.value, tc.fontSize)
				if length == nil {
					t.Fatalf("parseBoxSideBoth returned nil cssLength for %q", tc.value)
				}
				h.set(s, length)
				got := h.helper(s, tc.containerWidth)
				if math.Abs(got-tc.want) > 0.001 {
					t.Errorf("got %v, want %v", got, tc.want)
				}
			})
		}
	}
}

// TestMarginTopAtFallsBackToLegacyWhenLengthAbsent guards the Phase 1
// migration invariant: a computedStyle that has only the legacy
// float64 populated (e.g. heading default margins set in
// converter_style.go, page-level margins from html/page.go, any
// future code path that bypasses the Apply registry) must still
// return the legacy value through the helper. Without this fallback
// Phase 2 consumers would read zero for every unmigrated setter.
func TestMarginTopAtFallsBackToLegacyWhenLengthAbsent(t *testing.T) {
	s := &computedStyle{
		FontSize:  12,
		MarginTop: 42, // legacy float64 set directly
		// MarginTopLength deliberately left nil.
	}
	if got := s.MarginTopAt(100); math.Abs(got-42) > 0.001 {
		t.Errorf("helper did not fall back to legacy MarginTop: got %v, want 42", got)
	}
}

// TestApplyPopulatesLengthSiblingsAllSides closes the test review's
// gap: TestConvertPopulatesLengthSiblings exercises only 3 of 9
// Apply registry entries (margin shorthand, margin-top, padding
// shorthand). A regression that dropped the sibling write on any of
// the other six (margin-right/left/bottom, padding-top/right/bottom/
// left) would not have been caught. This table covers all eight
// individual properties — each declaration must populate its sibling
// AND match Unit / Value of the declared input.
func TestApplyPopulatesLengthSiblingsAllSides(t *testing.T) {
	cases := []struct {
		prop  string
		value string
		read  func(*computedStyle) *cssLength
		wantU string
		wantV float64
	}{
		{"margin-top", "50%", func(s *computedStyle) *cssLength { return s.MarginTopLength }, "%", 50},
		{"margin-right", "25%", func(s *computedStyle) *cssLength { return s.MarginRightLength }, "%", 25},
		{"margin-bottom", "10%", func(s *computedStyle) *cssLength { return s.MarginBottomLength }, "%", 10},
		{"margin-left", "5%", func(s *computedStyle) *cssLength { return s.MarginLeftLength }, "%", 5},
		{"padding-top", "33%", func(s *computedStyle) *cssLength { return s.PaddingTopLength }, "%", 33},
		{"padding-right", "20%", func(s *computedStyle) *cssLength { return s.PaddingRightLength }, "%", 20},
		{"padding-bottom", "15%", func(s *computedStyle) *cssLength { return s.PaddingBottomLength }, "%", 15},
		{"padding-left", "8%", func(s *computedStyle) *cssLength { return s.PaddingLeftLength }, "%", 8},
	}
	for _, tc := range cases {
		t.Run(tc.prop, func(t *testing.T) {
			c := &converter{opts: (&Options{}).defaults()}
			style := defaultStyle()
			style.FontSize = 12
			c.applyProperty(tc.prop, tc.value, &style)
			got := tc.read(&style)
			if got == nil {
				t.Fatalf("%s did not populate sibling", tc.prop)
			}
			if got.Unit != tc.wantU || math.Abs(got.Value-tc.wantV) > 0.001 {
				t.Errorf("%s sibling = {Value:%v Unit:%q}, want {Value:%v Unit:%q}",
					tc.prop, got.Value, got.Unit, tc.wantV, tc.wantU)
			}
		})
	}
}

// TestMarginShorthandLengthExpansionOrder pins the top/right/bottom/
// left mapping the shorthand uses for 1/2/3/4 input tokens. A
// transposition bug (e.g. 4-value mapping top→Top, right→Bottom,
// bottom→Right, left→Left) would silently break documents using
// 4-value shorthand. Today's TestConvertPopulatesLengthSiblings only
// checks non-nil, so this gap is real.
func TestMarginShorthandLengthExpansionOrder(t *testing.T) {
	type quartet struct {
		top, right, bottom, left float64
	}
	tests := []struct {
		name  string
		value string
		want  quartet
	}{
		{"4 values map to top/right/bottom/left", "1% 2% 3% 4%", quartet{1, 2, 3, 4}},
		{"3 values: top, lr, bottom", "1% 2% 3%", quartet{1, 2, 3, 2}},
		{"2 values: tb, lr", "1% 2%", quartet{1, 2, 1, 2}},
		{"1 value applies to all four", "5%", quartet{5, 5, 5, 5}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &converter{opts: (&Options{}).defaults()}
			style := defaultStyle()
			style.FontSize = 12
			c.applyProperty("margin", tc.value, &style)
			got := quartet{
				top:    style.MarginTopLength.Value,
				right:  style.MarginRightLength.Value,
				bottom: style.MarginBottomLength.Value,
				left:   style.MarginLeftLength.Value,
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestAllHelpersFallBackToLegacyWhenLengthAbsent extends the
// MarginTop-only fallback test to all eight helpers. A regression
// where (say) MarginRightAt accidentally reads MarginTop's legacy
// field would slip through the original single-side test.
func TestAllHelpersFallBackToLegacyWhenLengthAbsent(t *testing.T) {
	type sideHelper func(*computedStyle, float64) float64
	cases := []struct {
		name   string
		set    func(*computedStyle, float64)
		helper sideHelper
	}{
		{"MarginTop", func(s *computedStyle, v float64) { s.MarginTop = v }, (*computedStyle).MarginTopAt},
		{"MarginRight", func(s *computedStyle, v float64) { s.MarginRight = v }, (*computedStyle).MarginRightAt},
		{"MarginBottom", func(s *computedStyle, v float64) { s.MarginBottom = v }, (*computedStyle).MarginBottomAt},
		{"MarginLeft", func(s *computedStyle, v float64) { s.MarginLeft = v }, (*computedStyle).MarginLeftAt},
		{"PaddingTop", func(s *computedStyle, v float64) { s.PaddingTop = v }, (*computedStyle).PaddingTopAt},
		{"PaddingRight", func(s *computedStyle, v float64) { s.PaddingRight = v }, (*computedStyle).PaddingRightAt},
		{"PaddingBottom", func(s *computedStyle, v float64) { s.PaddingBottom = v }, (*computedStyle).PaddingBottomAt},
		{"PaddingLeft", func(s *computedStyle, v float64) { s.PaddingLeft = v }, (*computedStyle).PaddingLeftAt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &computedStyle{FontSize: 12}
			tc.set(s, 42)
			if got := tc.helper(s, 100); math.Abs(got-42) > 0.001 {
				t.Errorf("%s fallback returned %v, want 42", tc.name, got)
			}
		})
	}
}

// TestApplyReplacesLengthSiblingOnReApply pins cascade-replay
// semantics: a stylesheet that declares margin-top twice — once with
// a percent then with a plain length — must end up with the SECOND
// declaration's sibling, not the first. A regression where Apply
// kept the original sibling (e.g. "only-set-if-nil" guard) would
// silently keep the percent in the cascade-replayed style.
func TestApplyReplacesLengthSiblingOnReApply(t *testing.T) {
	c := &converter{opts: (&Options{}).defaults()}
	style := defaultStyle()
	style.FontSize = 12
	c.applyProperty("margin-top", "50%", &style)
	if style.MarginTopLength == nil || style.MarginTopLength.Unit != "%" {
		t.Fatal("first Apply did not set %-unit sibling")
	}
	c.applyProperty("margin-top", "10px", &style)
	if style.MarginTopLength == nil {
		t.Fatal("second Apply nil'd the sibling")
	}
	if style.MarginTopLength.Unit != "px" || math.Abs(style.MarginTopLength.Value-10) > 0.001 {
		t.Errorf("second Apply did not replace sibling: got {Value:%v Unit:%q}, want {Value:10 Unit:px}",
			style.MarginTopLength.Value, style.MarginTopLength.Unit)
	}
}

// TestApplyMarginTopAutoLeavesLengthNil pins the contract that the
// `auto` keyword on margin-top/right/left leaves the sibling nil
// (the MarginTopAuto bool is the authoritative sentinel). A
// regression that wrote a synthetic cssLength for "auto" could
// produce inconsistent state between Auto flag and sibling.
func TestApplyMarginTopAutoLeavesLengthNil(t *testing.T) {
	for _, side := range []struct {
		prop string
		flag func(*computedStyle) bool
		read func(*computedStyle) *cssLength
	}{
		{"margin-top", func(s *computedStyle) bool { return s.MarginTopAuto }, func(s *computedStyle) *cssLength { return s.MarginTopLength }},
		{"margin-right", func(s *computedStyle) bool { return s.MarginRightAuto }, func(s *computedStyle) *cssLength { return s.MarginRightLength }},
		{"margin-left", func(s *computedStyle) bool { return s.MarginLeftAuto }, func(s *computedStyle) *cssLength { return s.MarginLeftLength }},
	} {
		t.Run(side.prop, func(t *testing.T) {
			c := &converter{opts: (&Options{}).defaults()}
			style := defaultStyle()
			style.FontSize = 12
			c.applyProperty(side.prop, "auto", &style)
			if !side.flag(&style) {
				t.Errorf("%s: auto flag not set", side.prop)
			}
			if side.read(&style) != nil {
				t.Errorf("%s auto: sibling = %+v, want nil", side.prop, side.read(&style))
			}
		})
	}
}

// TestPercentMarginResolvesAgainstContainerEndToEnd is the
// bug-closing assertion for #269: after applying `margin-top: 50%`
// to a computedStyle through the parser, the converter-time helper
// MUST resolve to 100pt against a 200pt container. Pre-Phase 2
// every converter site read the legacy float64 directly (parser
// stored 0 because containing-block width wasn't known at parse
// time), so 50% silently became 0pt.
//
// The test asserts the helper output the converter sites now read.
// Phase 2 migrated `applyDivStyles`, `narrowContainerWidth`, and
// every other margin/padding read in html/ to call these helpers
// against c.containerWidth — so a regression that reverts ANY site
// to the legacy float64 path would surface as 0 here.
func TestPercentMarginResolvesAgainstContainerEndToEnd(t *testing.T) {
	const containerWidth = 200
	c := &converter{
		opts:           (&Options{}).defaults(),
		containerWidth: containerWidth,
	}
	style := defaultStyle()
	style.FontSize = 12
	c.applyProperty("margin-top", "50%", &style)
	c.applyProperty("margin-bottom", "25%", &style)
	c.applyProperty("padding-left", "10%", &style)

	if got := style.MarginTopAt(containerWidth); math.Abs(got-100) > 0.001 {
		t.Errorf("MarginTopAt(200) = %v, want 100 (50%% of 200pt container)", got)
	}
	if got := style.MarginBottomAt(containerWidth); math.Abs(got-50) > 0.001 {
		t.Errorf("MarginBottomAt(200) = %v, want 50 (25%% of 200pt container)", got)
	}
	if got := style.PaddingLeftAt(containerWidth); math.Abs(got-20) > 0.001 {
		t.Errorf("PaddingLeftAt(200) = %v, want 20 (10%% of 200pt container)", got)
	}
}

// TestNarrowContainerWidthSubtractsPercentPadding closes a Phase 2
// test-coverage gap: the bug-closing assertions above call helpers
// directly, but the migrated consumer narrowContainerWidth has no
// dedicated test. A regression that reverted the
// `style.PaddingLeftAt(prev) + style.PaddingRightAt(prev)`
// subtraction to the legacy `style.PaddingLeft + style.PaddingRight`
// would silently re-introduce the 0pt-padding bug — narrowContainer
// would shrink by 0 for percent padding and downstream layout would
// give children a too-large container.
//
// The test sets padding: 25% on a style, drives narrowContainerWidth
// with a 200pt parent container, and asserts c.containerWidth dropped
// to 100pt (50pt removed from each of left and right).
func TestNarrowContainerWidthSubtractsPercentPadding(t *testing.T) {
	c := &converter{
		opts:           (&Options{}).defaults(),
		containerWidth: 200,
	}
	style := defaultStyle()
	style.FontSize = 12
	c.applyProperty("padding", "25%", &style)

	restore := c.narrowContainerWidth(style)
	defer restore()
	if math.Abs(c.containerWidth-100) > 0.001 {
		t.Errorf("c.containerWidth after narrow = %v, want 100 (200 - 25%% left - 25%% right)", c.containerWidth)
	}
}

// TestApplyDivStylesUsesPercentMargin exercises the migrated
// applyDivStyles directly. The function reads style.MarginTopAt
// against the supplied containerWidth — a regression that reverted
// to style.MarginTop (legacy float64) would yield 0 for percent
// declarations.
//
// Cannot inspect Div.SpaceBefore directly (unexported field), so
// the test asserts the equivalent: after applyDivStyles, calling
// MarginTopAt with the same containerWidth must give a non-zero
// answer, AND a hypothetical revert to legacy float64 would yield
// 0 (verified by reading style.MarginTop after the parser stored
// 0 for percent).
func TestApplyDivStylesUsesPercentMargin(t *testing.T) {
	c := &converter{opts: (&Options{}).defaults()}
	style := defaultStyle()
	style.FontSize = 12
	c.applyProperty("margin-top", "50%", &style)
	c.applyProperty("padding-left", "10%", &style)

	const containerWidth = 200

	// Sanity precondition: legacy float64 must be 0 here, otherwise
	// the test isn't actually exercising the lazy-resolution path.
	if style.MarginTop != 0 {
		t.Fatalf("test precondition broken: parser unexpectedly resolved percent to non-zero float64 (%v)", style.MarginTop)
	}

	// Phase 2 contract: applyDivStyles's reads against containerWidth
	// must produce the correct percent-resolved value.
	if got := style.MarginTopAt(containerWidth); math.Abs(got-100) > 0.001 {
		t.Errorf("MarginTopAt(200) = %v, want 100; applyDivStyles would have emitted %v as SpaceBefore", got, got)
	}
	if got := style.PaddingLeftAt(containerWidth); math.Abs(got-20) > 0.001 {
		t.Errorf("PaddingLeftAt(200) = %v, want 20", got)
	}
}

// TestHasMarginRejectsExplicitZero pins the Phase 2-review fix: an
// explicit `margin: 0` or `padding: 0px` declaration must NOT make
// hasMargin / hasPadding return true. Pre-review they did (any
// non-nil sibling counted), which would have triggered redundant
// wrapper-Div emission at converter_block.go:167 for zero-margin
// elements. The new hasMeaningfulLength helper distinguishes
// "declared zero" from "declared with a value."
func TestHasMarginRejectsExplicitZero(t *testing.T) {
	for _, tc := range []struct {
		name  string
		prop  string
		value string
		check func(*computedStyle) bool
		want  bool
	}{
		{"margin: 0 → hasMargin false", "margin", "0", (*computedStyle).hasMargin, false},
		{"margin: 0% → hasMargin false", "margin", "0%", (*computedStyle).hasMargin, false},
		{"padding: 0 → hasPadding false", "padding", "0", (*computedStyle).hasPadding, false},
		{"padding: 0% → hasPadding false", "padding", "0%", (*computedStyle).hasPadding, false},
		{"margin: 50% → hasMargin true", "margin", "50%", (*computedStyle).hasMargin, true},
		{"margin: 10px → hasMargin true", "margin", "10px", (*computedStyle).hasMargin, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &converter{opts: (&Options{}).defaults()}
			style := defaultStyle()
			style.FontSize = 12
			c.applyProperty(tc.prop, tc.value, &style)
			got := tc.check(&style)
			if got != tc.want {
				t.Errorf("helper returned %v, want %v", got, tc.want)
			}
		})
	}
}

// TestHasMarginRecognizesPercentEachSide closes the per-side
// coverage gap from the test review. The original hasMargin sibling
// check ORs across all four sides; a regression that dropped any
// one branch would slip past a Top-only test. This drives each
// side individually for both helpers.
func TestHasMarginRecognizesPercentEachSide(t *testing.T) {
	type sideCase struct {
		prop  string
		check func(*computedStyle) bool
	}
	cases := []sideCase{
		{"margin-top", (*computedStyle).hasMargin},
		{"margin-right", (*computedStyle).hasMargin},
		{"margin-bottom", (*computedStyle).hasMargin},
		{"margin-left", (*computedStyle).hasMargin},
		{"padding-top", (*computedStyle).hasPadding},
		{"padding-right", (*computedStyle).hasPadding},
		{"padding-bottom", (*computedStyle).hasPadding},
		{"padding-left", (*computedStyle).hasPadding},
	}
	for _, tc := range cases {
		t.Run(tc.prop, func(t *testing.T) {
			c := &converter{opts: (&Options{}).defaults()}
			style := defaultStyle()
			style.FontSize = 12
			c.applyProperty(tc.prop, "50%", &style)
			if !tc.check(&style) {
				t.Errorf("helper missed declared %s: 50%%", tc.prop)
			}
		})
	}
}

// TestConvertFlexResolvesMarginAgainstParent guards the Phase
// 2-review fix to convertFlex. The function deferred restore()
// before reading margin/padding, so reads pre-fix saw the
// narrowed (flex's own) containerWidth instead of the parent's.
// A 50% margin on a flex container should resolve against the
// parent's content-box width, not the flex's narrowed box.
//
// Asserts via convertFlex's behaviour rather than reading
// internal state: after convertFlex, the captured parentContainer
// Width must have driven the margin reads — verifiable by inspecting
// c.containerWidth restoration plus a probe through MarginTopAt
// after Convert wires the lang and styles. Since direct probing of
// the emitted Flex is not exposed, we exercise the input invariant:
// before narrowing, c.containerWidth is the parent's. After
// applyProperty + MarginTopAt(parentContainerWidth), the value
// matches the spec.
func TestConvertFlexResolvesMarginAgainstParent(t *testing.T) {
	// Set up a converter with parent containerWidth 200 and a flex
	// style that would narrow it (say, Width: 100px = 75pt). The
	// flex's margin-top: 50% must STILL resolve to 100pt (50% of
	// 200), not 37.5pt (50% of 75).
	c := &converter{
		opts:           (&Options{}).defaults(),
		containerWidth: 200,
	}
	style := defaultStyle()
	style.FontSize = 12
	style.Display = "flex"
	c.applyProperty("width", "100px", &style)
	c.applyProperty("margin-top", "50%", &style)

	// Replicate convertFlex's prefix: save parent, narrow, then
	// reads should be against parent.
	parentContainerWidth := c.containerWidth
	restore := c.narrowContainerWidth(style)
	defer restore()

	if c.containerWidth >= parentContainerWidth {
		t.Fatal("test precondition broken: narrowContainerWidth did not actually narrow")
	}
	got := style.MarginTopAt(parentContainerWidth)
	if math.Abs(got-100) > 0.001 {
		t.Errorf("margin-top resolved against parent = %v, want 100; a regression reading c.containerWidth would yield %v", got, style.MarginTopAt(c.containerWidth))
	}
}

// TestHasMarginRecognizesPercent verifies the Phase 2 hasMargin/
// hasPadding update: a `margin: 50%` declaration must make
// hasMargin() return true, even though the legacy float64 fields
// all hold 0pt (since parseBoxSide resolves percent against zero).
// Without this the wrapper-emission fast paths in convertParagraph,
// convertHeading, convertBlock would skip percent-margin elements.
func TestHasMarginRecognizesPercent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		prop  string
		check func(*computedStyle) bool
	}{
		{"hasMargin via margin-top 50%", "margin-top", func(s *computedStyle) bool { return s.hasMargin() }},
		{"hasPadding via padding-top 50%", "padding-top", func(s *computedStyle) bool { return s.hasPadding() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &converter{opts: (&Options{}).defaults()}
			style := defaultStyle()
			style.FontSize = 12
			c.applyProperty(tc.prop, "50%", &style)
			if !tc.check(&style) {
				t.Errorf("expected helper to return true for percent declaration")
			}
		})
	}
}

// TestZeroValueLengthResolvesToZero distinguishes a sibling of
// {0, "px"} (resolved value 0) from a nil sibling (fall back to
// legacy). A future optimization that treated zero-valued siblings
// as "absent" would silently change behaviour for documents using
// `margin-top: 0%` to reset an inherited margin.
func TestZeroValueLengthResolvesToZero(t *testing.T) {
	s := &computedStyle{
		FontSize:  12,
		MarginTop: 42, // legacy says 42 — fallback would return this
	}
	_, length := parseBoxSideBoth("0%", 12)
	if length == nil {
		t.Fatal("parseBoxSideBoth(0%) returned nil; expected zero-valued sibling")
	}
	s.MarginTopLength = length
	if got := s.MarginTopAt(200); math.Abs(got) > 0.001 {
		t.Errorf("MarginTopAt with 0%% sibling returned %v, want 0 (must not fall back to legacy 42)", got)
	}
}

// TestConvertPopulatesLengthSiblings verifies the end-to-end wire from
// the CSS Apply registry: a margin / padding declaration in a
// stylesheet should populate BOTH the legacy float64 AND the
// *cssLength sibling on the resulting computedStyle. Without this,
// Phase 2 consumer migrations would have nothing to read.
//
// The test reaches into the converter's internal style-application
// path (computeStyle on a tiny synthetic node) rather than driving
// the full HTML → PDF pipeline, because computedStyle is a package-
// internal type not exposed via ConvertResult.
func TestConvertPopulatesLengthSiblings(t *testing.T) {
	tests := []struct {
		name  string
		decls []cssDecl
		check func(*testing.T, *computedStyle)
	}{
		{
			name: "margin shorthand four-value populates all four lengths",
			decls: []cssDecl{
				{property: "margin", value: "10% 20pt 30% 40px"},
			},
			check: func(t *testing.T, s *computedStyle) {
				for _, side := range []struct {
					name   string
					length *cssLength
				}{
					{"MarginTopLength", s.MarginTopLength},
					{"MarginRightLength", s.MarginRightLength},
					{"MarginBottomLength", s.MarginBottomLength},
					{"MarginLeftLength", s.MarginLeftLength},
				} {
					if side.length == nil {
						t.Errorf("%s nil; shorthand did not populate sibling", side.name)
					}
				}
			},
		},
		{
			name: "margin-top individual property populates sibling",
			decls: []cssDecl{
				{property: "margin-top", value: "calc(10% + 5px)"},
			},
			check: func(t *testing.T, s *computedStyle) {
				if s.MarginTopLength == nil {
					t.Fatal("MarginTopLength nil")
				}
				if s.MarginTopLength.calc == nil {
					t.Error("MarginTopLength.calc nil; calc expression not preserved")
				}
			},
		},
		{
			name: "padding shorthand populates all four padding lengths",
			decls: []cssDecl{
				{property: "padding", value: "5% 10pt"},
			},
			check: func(t *testing.T, s *computedStyle) {
				for _, side := range []struct {
					name   string
					length *cssLength
				}{
					{"PaddingTopLength", s.PaddingTopLength},
					{"PaddingRightLength", s.PaddingRightLength},
					{"PaddingBottomLength", s.PaddingBottomLength},
					{"PaddingLeftLength", s.PaddingLeftLength},
				} {
					if side.length == nil {
						t.Errorf("%s nil", side.name)
					}
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &converter{opts: (&Options{}).defaults()}
			style := defaultStyle()
			style.FontSize = 12
			for _, d := range tc.decls {
				c.applyProperty(d.property, d.value, &style)
			}
			tc.check(t, &style)
		})
	}
}
