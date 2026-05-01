// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"
import (
	"fmt"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// folio_paragraph_new creates a paragraph with a standard font and returns its handle.
//
//export folio_paragraph_new
func folio_paragraph_new(text *C.char, fontH C.uint64_t, fontSize C.double) C.uint64_t {
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return 0
	}
	p := layout.NewParagraph(C.GoString(text), f, float64(fontSize))
	return C.uint64_t(ht.store(p))
}

// folio_paragraph_new_embedded creates a paragraph with an embedded TrueType font.
//
//export folio_paragraph_new_embedded
func folio_paragraph_new_embedded(text *C.char, fontH C.uint64_t, fontSize C.double) C.uint64_t {
	ef, errCode := loadEmbeddedFont(fontH)
	if errCode != errOK {
		return 0
	}
	p := layout.NewParagraphEmbedded(C.GoString(text), ef, float64(fontSize))
	return C.uint64_t(ht.store(p))
}

// folio_paragraph_set_align sets the text alignment of a paragraph.
//
//export folio_paragraph_set_align
func folio_paragraph_set_align(pH C.uint64_t, align C.int32_t) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetAlign(layout.Align(align))
	return errOK
}

// folio_paragraph_set_leading sets the line spacing (leading) of a paragraph in points.
//
//export folio_paragraph_set_leading
func folio_paragraph_set_leading(pH C.uint64_t, leading C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetLeading(float64(leading))
	return errOK
}

// folio_paragraph_set_space_before sets the vertical spacing before a paragraph in points.
//
//export folio_paragraph_set_space_before
func folio_paragraph_set_space_before(pH C.uint64_t, pts C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetSpaceBefore(float64(pts))
	return errOK
}

// folio_paragraph_set_space_after sets the vertical spacing after a paragraph in points.
//
//export folio_paragraph_set_space_after
func folio_paragraph_set_space_after(pH C.uint64_t, pts C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetSpaceAfter(float64(pts))
	return errOK
}

// folio_paragraph_set_background sets the background color of a paragraph using RGB values in [0,1].
//
//export folio_paragraph_set_background
func folio_paragraph_set_background(pH C.uint64_t, r, g, b C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetBackground(layout.RGB(float64(r), float64(g), float64(b)))
	return errOK
}

// folio_paragraph_set_first_indent sets the first-line indentation of a paragraph in points.
//
//export folio_paragraph_set_first_indent
func folio_paragraph_set_first_indent(pH C.uint64_t, pts C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetFirstLineIndent(float64(pts))
	return errOK
}

// folio_paragraph_add_run appends a styled text run to a paragraph with the given font and color.
//
//export folio_paragraph_add_run
func folio_paragraph_add_run(pH C.uint64_t, text *C.char, fontH C.uint64_t, fontSize C.double, r, g, b C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	run := layout.TextRun{
		Text:     C.GoString(text),
		FontSize: float64(fontSize),
		Color:    layout.RGB(float64(r), float64(g), float64(b)),
	}
	// Determine font type from handle.
	v := ht.load(uint64(fontH))
	if v == nil {
		setLastError("invalid font handle")
		return errInvalidHandle
	}
	switch f := v.(type) {
	case *font.Standard:
		run.Font = f
	case *font.EmbeddedFont:
		run.Embedded = f
	default:
		setLastError(fmt.Sprintf("handle %d is not a font (type %T)", uint64(fontH), v))
		return errTypeMismatch
	}
	p.AddRun(run)
	return errOK
}

// folio_paragraph_set_direction sets the paragraph's text direction.
// Values: 0 = auto (detect from first strong character; default),
// 1 = left-to-right, 2 = right-to-left. Out-of-range values are
// treated as auto.
//
//export folio_paragraph_set_direction
func folio_paragraph_set_direction(pH C.uint64_t, dir C.int32_t) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetDirection(layoutDirectionFromC(dir))
	return errOK
}

// folio_paragraph_measure_lines reports how many wrapped lines the
// paragraph produces at the given maxWidth (in points). Returns -1 on
// invalid handle. Use this to make clamp/truncate decisions before
// calling folio_paragraph_split_after_line.
//
//export folio_paragraph_measure_lines
func folio_paragraph_measure_lines(pH C.uint64_t, maxWidth C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	return C.int32_t(p.MeasureLines(float64(maxWidth)))
}

// folio_paragraph_measure_height reports the rendered height (in
// points) of the paragraph wrapped at maxWidth. Excludes
// SpaceBefore/SpaceAfter so callers compose with their own pagination
// math. Returns a negative sentinel (NaN-like) on invalid handle:
// callers should check the error via folio_last_error.
//
//export folio_paragraph_measure_height
func folio_paragraph_measure_height(pH C.uint64_t, maxWidth C.double) C.double {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return -1
	}
	return C.double(p.MeasureHeight(float64(maxWidth)))
}

// folio_paragraph_split_after_line splits the paragraph after the
// first n rendered lines at maxWidth. On success, *outHead and *outTail
// receive paragraph handles (either may be 0 for the no-op halves):
// n <= 0 returns (0, full clone); n >= total lines returns (full clone, 0).
// The receiver is unchanged. Returned handles must be freed via
// folio_paragraph_free. outHead and outTail may not be NULL.
//
//export folio_paragraph_split_after_line
func folio_paragraph_split_after_line(pH C.uint64_t, n C.int32_t, maxWidth C.double, outHead, outTail *C.uint64_t) C.int32_t {
	if outHead == nil || outTail == nil {
		setLastError("outHead and outTail must be non-NULL")
		return errInvalidHandle
	}
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	head, tail := p.SplitAfterLine(int(n), float64(maxWidth))
	if head != nil {
		*outHead = C.uint64_t(ht.store(head))
	} else {
		*outHead = 0
	}
	if tail != nil {
		*outTail = C.uint64_t(ht.store(tail))
	} else {
		*outTail = 0
	}
	return errOK
}

// folio_paragraph_free removes a paragraph handle from the handle table.
//
//export folio_paragraph_free
func folio_paragraph_free(pH C.uint64_t) {
	ht.delete(uint64(pH))
}

// layoutDirectionFromC converts a C int32 direction code to
// layout.Direction. Unknown values are mapped to DirectionAuto so a
// consumer miscoding the constant does not end up with a silently
// reversed reading direction.
func layoutDirectionFromC(dir C.int32_t) layout.Direction {
	switch dir {
	case 1:
		return layout.DirectionLTR
	case 2:
		return layout.DirectionRTL
	default:
		return layout.DirectionAuto
	}
}

// loadParagraph retrieves a *layout.Paragraph from the handle table.
func loadParagraph(h C.uint64_t) (*layout.Paragraph, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid paragraph handle")
		return nil, errInvalidHandle
	}
	p, ok := v.(*layout.Paragraph)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a paragraph (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return p, errOK
}
