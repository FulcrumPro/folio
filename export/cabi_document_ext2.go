// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"
import (
	"os"
	"unsafe"

	"github.com/carlos7ags/folio/document"
	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

// ── File attachments ───────────────────────────────────────────────

//export folio_document_attach_file
func folio_document_attach_file(docH C.uint64_t, data unsafe.Pointer, length C.int32_t,
	fileName, mimeType, description, afRelationship *C.char) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	if data == nil || length <= 0 {
		setLastError("invalid attachment data")
		return errInvalidArg
	}
	goData := C.GoBytes(data, C.int(length))
	doc.AttachFile(document.FileAttachment{
		FileName:       C.GoString(fileName),
		MIMEType:       C.GoString(mimeType),
		Description:    C.GoString(description),
		AFRelationship: C.GoString(afRelationship),
		Data:           goData,
	})
	return errOK
}

// ── Inline HTML ────────────────────────────────────────────────────

//export folio_document_add_html
func folio_document_add_html(docH C.uint64_t, htmlStr *C.char) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	if err := doc.AddHTML(C.GoString(htmlStr), nil); err != nil {
		return setErr(errPDF, err)
	}
	return errOK
}

//export folio_document_add_html_with_options
func folio_document_add_html_with_options(docH C.uint64_t, htmlStr *C.char,
	defaultFontSize, pageWidth, pageHeight C.double,
	basePath, fallbackFontPath *C.char) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	opts := &foliohtml.Options{
		DefaultFontSize:  float64(defaultFontSize),
		PageWidth:        float64(pageWidth),
		PageHeight:       float64(pageHeight),
		FallbackFontPath: C.GoString(fallbackFontPath),
	}
	// fs.FS cannot cross the C boundary, so wrap basePath as os.DirFS at
	// the boundary. An empty basePath leaves BaseFS nil; relative asset
	// references in the HTML will then fail to resolve (callers must
	// either pass a basePath or inline assets via data: URIs).
	if bp := C.GoString(basePath); bp != "" {
		opts.BaseFS = os.DirFS(bp)
	}
	if err := doc.AddHTML(C.GoString(htmlStr), opts); err != nil {
		return setErr(errPDF, err)
	}
	return errOK
}

// ── Page-specific margins ──────────────────────────────────────────

//export folio_document_set_first_margins
func folio_document_set_first_margins(docH C.uint64_t, top, right, bottom, left C.double) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetFirstMargins(layout.Margins{
		Top: float64(top), Right: float64(right),
		Bottom: float64(bottom), Left: float64(left),
	})
	return errOK
}

//export folio_document_set_left_margins
func folio_document_set_left_margins(docH C.uint64_t, top, right, bottom, left C.double) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetLeftMargins(layout.Margins{
		Top: float64(top), Right: float64(right),
		Bottom: float64(bottom), Left: float64(left),
	})
	return errOK
}

//export folio_document_set_right_margins
func folio_document_set_right_margins(docH C.uint64_t, top, right, bottom, left C.double) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetRightMargins(layout.Margins{
		Top: float64(top), Right: float64(right),
		Bottom: float64(bottom), Left: float64(left),
	})
	return errOK
}

// folio_document_set_actual_text controls whether the writer wraps
// shaped Arabic words in /Span /ActualText marked-content sequences
// (ISO 32000-2 §14.9.4). Enabled by default. Disabling shaves a few
// dozen bytes per shaped Arabic word at the cost of copy/paste
// fidelity in PDF readers.
//
//export folio_document_set_actual_text
func folio_document_set_actual_text(docH C.uint64_t, enabled C.int32_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetActualText(enabled != 0)
	return errOK
}
