// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"
import (
	"bytes"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

// folio_html_to_pdf converts an HTML string to a PDF file at the given output path.
//
//export folio_html_to_pdf
func folio_html_to_pdf(htmlStr *C.char, outputPath *C.char) C.int32_t {
	doc, err := htmlToDocument(C.GoString(htmlStr), 0, 0)
	if err != errOK {
		return err
	}
	if saveErr := doc.Save(C.GoString(outputPath)); saveErr != nil {
		return setErr(errIO, saveErr)
	}
	return errOK
}

// folio_html_to_buffer converts an HTML string to an in-memory PDF buffer with optional page dimensions.
//
//export folio_html_to_buffer
func folio_html_to_buffer(htmlStr *C.char, pageWidth, pageHeight C.double) C.uint64_t {
	doc, err := htmlToDocument(C.GoString(htmlStr), float64(pageWidth), float64(pageHeight))
	if err != errOK {
		return 0
	}
	var buf bytes.Buffer
	if _, writeErr := doc.WriteTo(&buf); writeErr != nil {
		setLastError(writeErr.Error())
		return 0
	}
	return C.uint64_t(ht.store(newCBuffer(buf.Bytes())))
}

// folio_html_convert converts an HTML string to a document handle for further manipulation.
//
//export folio_html_convert
func folio_html_convert(htmlStr *C.char, pageWidth, pageHeight C.double) C.uint64_t {
	doc, err := htmlToDocument(C.GoString(htmlStr), float64(pageWidth), float64(pageHeight))
	if err != errOK {
		return 0
	}
	return C.uint64_t(ht.store(doc))
}

// htmlToDocument converts HTML to a Document ready for save/write.
func htmlToDocument(htmlStr string, pageWidth, pageHeight float64) (*document.Document, C.int32_t) {
	opts := &html.Options{}
	if pageWidth > 0 {
		opts.PageWidth = pageWidth
	}
	if pageHeight > 0 {
		opts.PageHeight = pageHeight
	}

	result, err := html.ConvertFull(htmlStr, opts)
	if err != nil {
		setLastError(err.Error())
		return nil, errPDF
	}

	// Determine page size.
	ps := document.PageSizeLetter
	if pageWidth > 0 && pageHeight > 0 {
		ps = document.PageSize{Width: pageWidth, Height: pageHeight}
	}
	// Resolve @page geometry + orientation-only swap + margin percentages /
	// calc through the shared helper so this path matches AddHTML (B-1).
	if pc := result.PageConfig; pc != nil {
		w, h, _ := pc.Resolve(ps.Width, ps.Height)
		ps = document.PageSize{Width: w, Height: h}
	}

	doc := document.NewDocument(ps)

	// Apply @page margins (default + :first/:left/:right overrides).
	if pc := result.PageConfig; pc != nil {
		if pc.HasMargins {
			doc.SetMargins(layout.Margins{
				Top: pc.MarginTop, Right: pc.MarginRight,
				Bottom: pc.MarginBottom, Left: pc.MarginLeft,
			})
		}
		if pc.First != nil && pc.First.HasMargins {
			doc.SetFirstMargins(layout.Margins{
				Top: pc.First.Top, Right: pc.First.Right,
				Bottom: pc.First.Bottom, Left: pc.First.Left,
			})
		}
		if pc.Left != nil && pc.Left.HasMargins {
			doc.SetLeftMargins(layout.Margins{
				Top: pc.Left.Top, Right: pc.Left.Right,
				Bottom: pc.Left.Bottom, Left: pc.Left.Left,
			})
		}
		if pc.Right != nil && pc.Right.HasMargins {
			doc.SetRightMargins(layout.Margins{
				Top: pc.Right.Top, Right: pc.Right.Right,
				Bottom: pc.Right.Bottom, Left: pc.Right.Left,
			})
		}
	}

	// Apply @page margin boxes (page numbers, headers/footers).
	if result.MarginBoxes != nil {
		doc.SetMarginBoxes(result.MarginBoxes)
	}
	if result.FirstMarginBoxes != nil {
		doc.SetFirstMarginBoxes(result.FirstMarginBoxes)
	}
	if result.LeftMarginBoxes != nil {
		doc.SetLeftMarginBoxes(result.LeftMarginBoxes)
	}
	if result.RightMarginBoxes != nil {
		doc.SetRightMarginBoxes(result.RightMarginBoxes)
	}

	// Apply metadata.
	if result.Metadata.Title != "" {
		doc.Info.Title = result.Metadata.Title
	}
	if result.Metadata.Author != "" {
		doc.Info.Author = result.Metadata.Author
	}

	for _, e := range result.Elements {
		doc.Add(e)
	}
	for _, abs := range result.Absolutes {
		doc.AddAbsoluteWithOpts(abs.Element, abs.X, abs.Y, abs.Width, layout.AbsoluteOpts{
			RightAligned: abs.RightAligned,
			ZIndex:       abs.ZIndex,
			PageIndex:    -1,
			Fixed:        abs.Fixed,
		})
	}

	return doc, errOK
}
