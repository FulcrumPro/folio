// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/carlos7ags/folio/font"
)

func newTestRenderer() *Renderer {
	r := NewRenderer(612, 792, Margins{Top: 72, Bottom: 72, Left: 72, Right: 72})
	for range 20 {
		r.Add(NewParagraph("content content content", font.Helvetica, 12))
	}
	return r
}

// TestRenderContextCancelled verifies the layout pass honors a cancelled
// context at the element boundary and returns ctx.Err() with no pages.
func TestRenderContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pages, err := newTestRenderer().RenderContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if pages != nil {
		t.Error("expected nil pages on cancellation")
	}
}

// TestRenderContextDeadline verifies a passed deadline is reported.
func TestRenderContextDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()

	_, err := newTestRenderer().RenderContext(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context.DeadlineExceeded, got %v", err)
	}
}

// TestRenderContextBackgroundMatchesRender confirms RenderContext with a live
// context produces the same page count as the plain Render path.
func TestRenderContextBackgroundMatchesRender(t *testing.T) {
	want := newTestRenderer().Render()

	pages, err := newTestRenderer().RenderContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != len(want) {
		t.Errorf("RenderContext pages = %d, Render pages = %d", len(pages), len(want))
	}
}
