// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "testing"

func TestSubstituteCountersResolvesPageAndPages(t *testing.T) {
	got := substituteCounters("Page "+CounterPagePlaceholder+" of "+CounterPagesPlaceholder, 2, 7)
	want := "Page 3 of 7"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstituteCountersIsNoopWhenAbsent(t *testing.T) {
	in := "no placeholders here"
	if got := substituteCounters(in, 0, 5); got != in {
		t.Errorf("expected unchanged %q, got %q", in, got)
	}
}

func TestExpandCountersForMeasureUsesDigitReservation(t *testing.T) {
	got := expandCountersForMeasure(CounterPagePlaceholder)
	if got != counterMeasureDigits {
		t.Errorf("page reservation: got %q, want %q", got, counterMeasureDigits)
	}
	got = expandCountersForMeasure(CounterPagesPlaceholder)
	if got != counterMeasureDigits {
		t.Errorf("pages reservation: got %q, want %q", got, counterMeasureDigits)
	}
}

func TestHasCounterPlaceholderDetectsBoth(t *testing.T) {
	if !hasCounterPlaceholder("Page " + CounterPagePlaceholder) {
		t.Error("page placeholder should be detected")
	}
	if !hasCounterPlaceholder(CounterPagesPlaceholder + " total") {
		t.Error("pages placeholder should be detected")
	}
	if hasCounterPlaceholder("plain text") {
		t.Error("plain text should not be flagged")
	}
}

func TestFormatCounterStyles(t *testing.T) {
	cases := []struct {
		n     int
		style string
		want  string
	}{
		{3, "decimal", "3"},
		{3, "", "3"},
		{3, "decimal-leading-zero", "03"},
		{12, "decimal-leading-zero", "12"},
		{4, "lower-roman", "iv"},
		{4, "upper-roman", "IV"},
		{1949, "upper-roman", "MCMXLIX"},
		{1, "lower-alpha", "a"},
		{26, "lower-alpha", "z"},
		{27, "lower-alpha", "aa"},
		{2, "upper-alpha", "B"},
		{5, "no-such-style", "5"}, // unknown → decimal fallback
	}
	for _, c := range cases {
		if got := formatCounter(c.n, c.style); got != c.want {
			t.Errorf("formatCounter(%d, %q) = %q, want %q", c.n, c.style, got, c.want)
		}
	}
}

func TestFormatCounterEdgeCases(t *testing.T) {
	cases := []struct {
		n     int
		style string
		want  string
	}{
		// Roman has no zero/negative form → decimal fallback (N-c).
		{0, "upper-roman", "0"},
		{-3, "lower-roman", "-3"},
		// Largest commonly cited Roman value.
		{4000, "upper-roman", "MMMM"},
		// Bijective base-26 alpha boundaries (N-c).
		{52, "lower-alpha", "az"},
		{53, "lower-alpha", "ba"},
		{52, "upper-alpha", "AZ"},
		{53, "upper-alpha", "BA"},
		// Alpha has no zero/negative form → decimal fallback.
		{0, "lower-alpha", "0"},
		{-1, "upper-alpha", "-1"},
	}
	for _, c := range cases {
		if got := formatCounter(c.n, c.style); got != c.want {
			t.Errorf("formatCounter(%d, %q) = %q, want %q", c.n, c.style, got, c.want)
		}
	}
}

func TestCounterPlaceholderRejectsDelimiterInStyle(t *testing.T) {
	// A style argument containing the placeholder's own delimiters must not
	// leak into the emitted token (N-a); it falls back to the style-less
	// canonical form.
	for _, bad := range []string{"upper-roman)", "x}y", "}", ")"} {
		if got := CounterPlaceholder("page", bad); got != CounterPagePlaceholder {
			t.Errorf("CounterPlaceholder(page, %q) = %q, want %q", bad, got, CounterPagePlaceholder)
		}
	}
}

func TestSubstituteCountersWithStyle(t *testing.T) {
	// counter(page, upper-roman) on page index 2 (1-based 3) → "III".
	in := "Page " + CounterPlaceholder("page", "upper-roman") +
		" / " + CounterPlaceholder("pages", "upper-roman")
	got := substituteCounters(in, 2, 7)
	want := "Page III / VII"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Mixed styled + unstyled in one string.
	mixed := CounterPlaceholder("page", "") + "-" + CounterPlaceholder("page", "lower-alpha")
	if got := substituteCounters(mixed, 0, 1); got != "1-a" {
		t.Errorf("mixed = %q, want %q", got, "1-a")
	}
}

func TestCounterPlaceholderNormalizesDecimal(t *testing.T) {
	// decimal / empty style collapse to the canonical style-less token so
	// existing emitters and back-compat behavior are preserved.
	if got := CounterPlaceholder("page", "decimal"); got != CounterPagePlaceholder {
		t.Errorf("decimal style: got %q, want %q", got, CounterPagePlaceholder)
	}
	if got := CounterPlaceholder("pages", ""); got != CounterPagesPlaceholder {
		t.Errorf("empty style: got %q, want %q", got, CounterPagesPlaceholder)
	}
}

func TestSubstituteCountersIgnoresUnknownCounterTokens(t *testing.T) {
	// A future or user-defined counter name (e.g. {counter(chapter)}) that
	// looks like a placeholder but isn't one of the two known names must
	// pass through unchanged — substitution only resolves page/pages.
	in := "see {counter(chapter)} for details"
	if got := substituteCounters(in, 0, 5); got != in {
		t.Errorf("unknown counter token should pass through: got %q, want %q", got, in)
	}
	if got := expandCountersForMeasure(in); got != in {
		t.Errorf("unknown counter token should not be expanded: got %q, want %q", got, in)
	}
	if hasCounterPlaceholder(in) {
		t.Error("unknown counter token should not register as a placeholder")
	}
}
