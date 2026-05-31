// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func paragraphsHTML(n int) string {
	var b strings.Builder
	b.WriteString("<html><body>")
	for range n {
		b.WriteString("<p>x</p>")
	}
	b.WriteString("</body></html>")
	return b.String()
}

// TestConvertWithContextCancelled verifies the tree walk honors a cancelled
// context and returns context.Canceled.
func TestConvertWithContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	elems, err := ConvertWithContext(ctx, paragraphsHTML(50), &Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if elems != nil {
		t.Error("expected nil elements on cancellation")
	}
}

// TestConvertFullWithContextDeadline verifies a passed deadline aborts
// conversion with context.DeadlineExceeded and a nil result.
func TestConvertFullWithContextDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()

	res, err := ConvertFullWithContext(ctx, paragraphsHTML(50), &Options{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context.DeadlineExceeded, got %v", err)
	}
	if res != nil {
		t.Error("expected nil result on cancellation")
	}
}

// TestConvertBackgroundContextOK confirms the non-context entry points (which
// delegate with context.Background) and a live context still convert.
func TestConvertBackgroundContextOK(t *testing.T) {
	elems, err := Convert(paragraphsHTML(5), &Options{})
	if err != nil {
		t.Fatalf("Convert: unexpected error: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("Convert: expected elements")
	}

	elems2, err := ConvertWithContext(context.Background(), paragraphsHTML(5), &Options{})
	if err != nil {
		t.Fatalf("ConvertWithContext: unexpected error: %v", err)
	}
	if len(elems2) != len(elems) {
		t.Errorf("ConvertWithContext produced %d elements, Convert %d", len(elems2), len(elems))
	}
}
