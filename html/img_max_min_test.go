// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"testing"
)

// onePxPNG is a 1×1 PNG, base64-encoded. Reused from the existing
// object-fit tests for the same reason: small enough to inline,
// decodes to a real image without shipping a fixture.
const onePxPNG = `data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQI12NgAAIABQABNjN9GQAAAABJRu5ErkJggg==`

// TestImgConvertAcceptsMaxMinConstraints is a smoke test that
// confirms the converter accepts the four new CSS constraints
// (max-width, max-height, min-width, min-height) on <img> elements
// without error and produces a non-empty element list. The unit-level
// contract — that the constraints actually clamp the rendered size —
// is locked in by TestImageElementMaxConstraints / MinConstraints in
// layout/image_max_min_test.go.
//
// The end-to-end inspection is awkward here: <img> in HTML defaults
// to inline display, and the converter wraps it in a paragraph
// regardless of `display: block` (the dispatch is per-atom in
// convertInlineElement, before CSS computed style takes effect for
// the inline-vs-block decision). The constraint reaches the inline
// ImageElement and clamps its resolveSize at draw time, but the
// outer block element exposed to the html test surface is the
// paragraph wrapper — its dimensions reflect the paragraph layout,
// not the image. The unit test reaches the same resolveSize call
// and asserts on its return value directly.
func TestImgConvertAcceptsMaxMinConstraints(t *testing.T) {
	tests := []string{
		`<img style="max-width: 50px" src="` + onePxPNG + `">`,
		`<img style="max-height: 32px" src="` + onePxPNG + `">`,
		`<img style="max-width: 180px; max-height: 32px" src="` + onePxPNG + `">`,
		`<img style="min-width: 100px" src="` + onePxPNG + `">`,
		`<img style="min-height: 100px" src="` + onePxPNG + `">`,
		`<img style="width: 10px; min-width: 50px" src="` + onePxPNG + `">`,
	}
	for _, src := range tests {
		t.Run(src, func(t *testing.T) {
			elems, err := Convert(src, nil)
			if err != nil {
				t.Errorf("Convert(%q) error: %v", src, err)
			}
			if len(elems) == 0 {
				t.Errorf("Convert(%q) returned no elements", src)
			}
		})
	}
}
