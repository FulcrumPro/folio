// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"errors"
	"strings"
	"testing"
)

func manyParagraphs(n int) string {
	var b strings.Builder
	b.WriteString("<html><body>")
	for range n {
		b.WriteString("<p>x</p>")
	}
	b.WriteString("</body></html>")
	return b.String()
}

func nestedDivs(depth int) string {
	return "<html><body>" + strings.Repeat("<div>", depth) + "x" +
		strings.Repeat("</div>", depth) + "</body></html>"
}

// TestMaxElementsExceeded verifies conversion fails closed with a typed
// *LimitError once the element budget is crossed.
func TestMaxElementsExceeded(t *testing.T) {
	_, err := Convert(manyParagraphs(50), &Options{MaxElements: 10})

	var le *LimitError
	if !errors.As(err, &le) {
		t.Fatalf("want *LimitError, got %T: %v", err, err)
	}
	if le.Kind != LimitElements {
		t.Errorf("Kind = %v, want LimitElements", le.Kind)
	}
	if le.Limit != 10 {
		t.Errorf("Limit = %d, want 10", le.Limit)
	}
}

// TestMaxElementsUnlimitedByDefault confirms the zero value preserves the
// historical unbounded behavior.
func TestMaxElementsUnlimitedByDefault(t *testing.T) {
	elems, err := Convert(manyParagraphs(50), &Options{})
	if err != nil {
		t.Fatalf("unexpected error with MaxElements unset: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements to be produced when unbounded")
	}
}

// TestMaxDepthExceeded verifies deeply nested input trips the depth guard.
func TestMaxDepthExceeded(t *testing.T) {
	_, err := Convert(nestedDivs(40), &Options{MaxDepth: 5})

	var le *LimitError
	if !errors.As(err, &le) {
		t.Fatalf("want *LimitError, got %T: %v", err, err)
	}
	if le.Kind != LimitDepth {
		t.Errorf("Kind = %v, want LimitDepth", le.Kind)
	}
	if le.Limit != 5 {
		t.Errorf("Limit = %d, want 5", le.Limit)
	}
}

// TestMaxDepthUnlimitedByDefault confirms deep nesting is allowed when the
// guard is off.
func TestMaxDepthUnlimitedByDefault(t *testing.T) {
	if _, err := Convert(nestedDivs(40), &Options{}); err != nil {
		t.Fatalf("unexpected error with MaxDepth unset: %v", err)
	}
}

// TestLimitErrorFromConvertFull verifies the ConvertFull entry point also
// fails closed and discards the partial result.
func TestLimitErrorFromConvertFull(t *testing.T) {
	res, err := ConvertFull(manyParagraphs(50), &Options{MaxElements: 5})
	if res != nil {
		t.Error("expected a nil result when a limit is hit")
	}
	var le *LimitError
	if !errors.As(err, &le) {
		t.Fatalf("want *LimitError, got %T: %v", err, err)
	}
}

// TestLimitErrorDistinctFromOtherKinds guards the taxonomy boundary: a
// LimitError must not be mistaken for an asset or parse fault.
func TestLimitErrorDistinctFromOtherKinds(t *testing.T) {
	_, err := Convert(manyParagraphs(50), &Options{MaxElements: 5})

	var ae *AssetError
	var pe *ParseError
	if errors.As(err, &ae) || errors.As(err, &pe) {
		t.Error("a LimitError must not match *AssetError or *ParseError")
	}
}

// TestLimitErrorMessages pins the rendered messages and kind labels.
func TestLimitErrorMessages(t *testing.T) {
	if got := (&LimitError{Kind: LimitElements, Limit: 100}).Error(); got != "folio/html: element count exceeded limit of 100" {
		t.Errorf("elements message: %q", got)
	}
	if got := (&LimitError{Kind: LimitDepth, Limit: 7}).Error(); got != "folio/html: nesting depth exceeded limit of 7" {
		t.Errorf("depth message: %q", got)
	}
	if LimitElements.String() != "elements" || LimitDepth.String() != "depth" {
		t.Error("unexpected LimitKind.String()")
	}
}
