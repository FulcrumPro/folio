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
