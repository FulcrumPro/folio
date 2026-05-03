// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
	goimage "image"
	"image/jpeg"
	"math"
	"testing"

	folioimage "github.com/carlos7ags/folio/image"
)

// makeImage creates a JPEG of the requested pixel dimensions for
// max/min constraint tests. Aspect ratio is widthPx/heightPx.
func makeImage(t *testing.T, widthPx, heightPx int) *folioimage.Image {
	t.Helper()
	img := goimage.NewRGBA(goimage.Rect(0, 0, widthPx, heightPx))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	fimg, err := folioimage.NewJPEG(buf.Bytes())
	if err != nil {
		t.Fatalf("folio.NewJPEG: %v", err)
	}
	return fimg
}

// TestImageElementMaxConstraints covers the canonical matrix from
// issue #291's spike doc. Pre-fix max-width / max-height were
// silently dropped at the converter level; even if threaded through,
// ImageElement had no field to receive them.
func TestImageElementMaxConstraints(t *testing.T) {
	tests := []struct {
		name   string
		imgW   int
		imgH   int
		maxW   float64
		maxH   float64
		canvas float64
		wantW  float64
		wantH  float64
	}{
		{
			name: "wide banner clamped by max-width",
			imgW: 2000, imgH: 500,
			maxW:   180,
			canvas: 540,
			wantW:  180, wantH: 45,
		},
		{
			name: "square clamped by max-height",
			imgW: 200, imgH: 200,
			maxH:   32,
			canvas: 540,
			wantW:  32, wantH: 32,
		},
		{
			name: "wide image, both bounds set, max-width wins",
			imgW: 1000, imgH: 100,
			maxW: 180, maxH: 32,
			canvas: 540,
			wantW:  180, wantH: 18,
		},
		{
			name: "tall image, both bounds set, max-height wins",
			imgW: 100, imgH: 800,
			maxW: 180, maxH: 32,
			canvas: 540,
			wantW:  4, wantH: 32,
		},
		{
			name: "small image, max-width wider than intrinsic — no upscale",
			imgW: 50, imgH: 50,
			maxW:   180,
			canvas: 540,
			// auto-auto path scales image to fill canvas (540pt wide), then
			// max-width clamps to 180pt. The constraint is an upper bound only:
			// a 50px image cannot upscale beyond its intrinsic size in the
			// "no explicit width or height" path, but the fill-to-canvas
			// behaviour predates this fix and is preserved.
			wantW: 180, wantH: 180,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fimg := makeImage(t, tc.imgW, tc.imgH)
			ie := NewImageElement(fimg).SetMaxSize(tc.maxW, tc.maxH)
			gotW, gotH := ie.resolveSize(tc.canvas)
			if math.Abs(gotW-tc.wantW) > 0.5 {
				t.Errorf("width = %.2f, want %.2f", gotW, tc.wantW)
			}
			if math.Abs(gotH-tc.wantH) > 0.5 {
				t.Errorf("height = %.2f, want %.2f", gotH, tc.wantH)
			}
		})
	}
}

// TestImageElementMaxNeverUpscales verifies that max-* is purely an
// upper bound — when the explicit width is already smaller than max,
// the image keeps its smaller size.
func TestImageElementMaxNeverUpscales(t *testing.T) {
	fimg := makeImage(t, 200, 200)
	// Explicit 50pt width, max-width 180pt — the explicit wins (smaller).
	ie := NewImageElement(fimg).SetSize(50, 0).SetMaxSize(180, 0)
	gotW, gotH := ie.resolveSize(540)
	if gotW != 50 {
		t.Errorf("width = %.2f, want 50 (explicit wins over max)", gotW)
	}
	if gotH != 50 {
		t.Errorf("height = %.2f, want 50 (aspect-preserved)", gotH)
	}
}

// TestImageElementMinConstraints covers the symmetric case for
// min-width / min-height. min-* is a lower bound that wins over
// max-* when they conflict per CSS 2.1 §10.7.
func TestImageElementMinConstraints(t *testing.T) {
	t.Run("min-width upscales explicit smaller width", func(t *testing.T) {
		fimg := makeImage(t, 200, 200)
		ie := NewImageElement(fimg).SetSize(50, 0).SetMinSize(100, 0)
		gotW, gotH := ie.resolveSize(540)
		if gotW != 100 {
			t.Errorf("width = %.2f, want 100 (min upscales)", gotW)
		}
		if gotH != 100 {
			t.Errorf("height = %.2f, want 100 (aspect-preserved)", gotH)
		}
	})
	t.Run("min wins over max conflict", func(t *testing.T) {
		fimg := makeImage(t, 200, 200)
		// max-width: 50, min-width: 100 — spec says min wins.
		ie := NewImageElement(fimg).SetSize(80, 0).SetMaxSize(50, 0).SetMinSize(100, 0)
		gotW, _ := ie.resolveSize(540)
		if gotW != 100 {
			t.Errorf("width = %.2f, want 100 (min beats max)", gotW)
		}
	})
	t.Run("min-height upscales", func(t *testing.T) {
		fimg := makeImage(t, 100, 100)
		ie := NewImageElement(fimg).SetSize(0, 20).SetMinSize(0, 50)
		_, gotH := ie.resolveSize(540)
		if gotH != 50 {
			t.Errorf("height = %.2f, want 50 (min-height upscales)", gotH)
		}
	})
}

// TestImageElementContainerStillWins guards that the container's
// available width remains the absolute upper bound, even when
// min-width would push past it. CSS 2.1 §10.4 resolves against the
// containing block.
func TestImageElementContainerStillWins(t *testing.T) {
	fimg := makeImage(t, 100, 100)
	// min-width: 600pt with a 200pt canvas — container caps at 200.
	ie := NewImageElement(fimg).SetMinSize(600, 0)
	gotW, _ := ie.resolveSize(200)
	if gotW > 200.5 {
		t.Errorf("width = %.2f, must not exceed canvas 200", gotW)
	}
}

// TestImageElementMaxClampsBoxBeforeObjectFit covers the reviewer's
// MEDIUM finding: pre-fix the object-fit branch returned early
// before the max/min clamps fired, so an image with explicit
// dimensions AND object-fit AND a max-width kept the unconstrained
// box. Post-fix the clamps run on (w, h) before the object-fit
// dispatch, so the box is constrained first then object-fit fits
// inside the constrained box.
//
// Per CSS Images L3 §3.2 + CSS 2.1 §10.7, max-* applies to the
// content box of replaced elements; object-fit modifies how the
// image content fills that box.
func TestImageElementMaxClampsBoxBeforeObjectFit(t *testing.T) {
	fimg := makeImage(t, 200, 200)
	// Author asks for 200×200 box with cover, but max-width 50pt
	// caps the box. The cover-fit then operates on the 50×50 box.
	ie := NewImageElement(fimg).
		SetSize(200, 200).
		SetObjectFit("cover").
		SetMaxSize(50, 0)
	gotW, gotH := ie.resolveSize(540)
	if gotW > 50.5 {
		t.Errorf("box width = %.2f, want ≤ 50 (max-width caps box)", gotW)
	}
	// Cover-fit on a square image inside a 50×50 box leaves 50×50.
	if gotH > 50.5 {
		t.Errorf("box height = %.2f, want ≤ 50 (cover-fit on square)", gotH)
	}
}

// TestImageElementMinHeightDroppedByContainerClamp documents the
// intentional spec behaviour from CSS 2.1 §10.4: the container
// width is the absolute outer bound for replaced elements, even
// against min-height that would push the image past it. This test
// pins the contract so a future "fix" doesn't accidentally let
// images bleed past their container.
func TestImageElementMinHeightDroppedByContainerClamp(t *testing.T) {
	fimg := makeImage(t, 100, 100)
	// min-height: 300pt with a 200pt-wide canvas. Auto-auto
	// produces 200×200, min-height upscales to 300×300, container
	// clamp resets to 200×200. min-height is silently dropped
	// because honouring it would require overflowing the container
	// width — which the spec forbids.
	ie := NewImageElement(fimg).SetMinSize(0, 300)
	gotW, gotH := ie.resolveSize(200)
	if gotW > 200.5 {
		t.Errorf("width = %.2f, must not exceed canvas 200", gotW)
	}
	if gotH > 200.5 {
		t.Errorf("height = %.2f, must not exceed container-clamped square", gotH)
	}
}

// TestImageElementZeroBoundsAreUnbounded confirms the API contract
// that 0 means "no constraint" on either axis. Calling SetMaxSize(0, 0)
// is equivalent to never calling it.
func TestImageElementZeroBoundsAreUnbounded(t *testing.T) {
	fimg := makeImage(t, 2000, 500)
	withZero := NewImageElement(fimg).SetMaxSize(0, 0)
	withoutCall := NewImageElement(fimg)

	wZ, hZ := withZero.resolveSize(540)
	wN, hN := withoutCall.resolveSize(540)
	if math.Abs(wZ-wN) > 0.01 || math.Abs(hZ-hN) > 0.01 {
		t.Errorf("zero bounds should match no-call: (%.2f, %.2f) vs (%.2f, %.2f)", wZ, hZ, wN, hN)
	}
}
