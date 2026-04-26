// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "strconv"

// Fallback wraps an ordered list of EmbeddedFont pointers used to render
// mixed-script text without forcing the caller to pre-split runs by font.
// It is a coverage dispatcher only: PickFace returns the first face whose
// cmap covers a given rune, falling back to the first face when nothing
// covers it. Segmentation, shaping, and PDF emission stay in the layers
// that already own them.
//
// Pointer identity matters. Fallback stores the original *EmbeddedFont
// pointers verbatim and never wraps or copies them. Subsetting,
// ToUnicode CMap generation, and the page-level font resource map all
// dedupe by pointer (one Type0 dict per *EmbeddedFont per document), so
// passing the same Fallback to many paragraphs aggregates used glyphs on
// the shared faces instead of inflating the resource count.
//
// Concurrency: Fallback methods are read-only and safe to call from
// multiple goroutines. Each underlying EmbeddedFont still carries the
// non-concurrent invariant documented on its type.
type Fallback struct {
	faces []*EmbeddedFont
}

// NewFallback returns a Fallback over the given faces, in priority order.
// PickFace tries faces[0] first, then faces[1], and so on. Panics if no
// faces are provided: a fallback with no faces has no defined behavior
// and almost certainly indicates a caller error.
func NewFallback(faces ...*EmbeddedFont) *Fallback {
	if len(faces) == 0 {
		panic("font.NewFallback: at least one face is required")
	}
	for i, f := range faces {
		if f == nil {
			panic("font.NewFallback: nil face at index " + strconv.Itoa(i))
		}
	}
	return &Fallback{faces: faces}
}

// Faces returns the underlying face slice in priority order. The returned
// slice aliases the Fallback's storage; callers must not mutate it.
func (fb *Fallback) Faces() []*EmbeddedFont {
	return fb.faces
}

// PickFace returns the first face that has a glyph for r. If no face
// covers r, the first face is returned so the caller still has something
// to render with (the missing glyph will appear as .notdef tofu, which
// is the conventional indicator that the document needs a broader
// fallback chain).
func (fb *Fallback) PickFace(r rune) *EmbeddedFont {
	for _, ef := range fb.faces {
		if ef.face.GlyphIndex(r) != 0 {
			return ef
		}
	}
	return fb.faces[0]
}
