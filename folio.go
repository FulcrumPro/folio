// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package folio is the root façade for the Folio PDF toolkit. It re-exports
// the most commonly used types and constructors from the document package so
// callers can depend on a single stable import path for the common
// "build a document, then write it" flow:
//
//	doc := folio.NewDocument(folio.PageSizeA4)
//	// ... add content ...
//	_, err := doc.WriteTo(w)
//
// The exported names are type aliases and function/value re-exports, so they
// are fully interchangeable with the document package's own identifiers and
// carry the complete method set — including methods added to
// document.Document over time.
//
// This façade intentionally covers only the document-construction surface.
// Specialized APIs keep their own packages: HTML conversion in
// github.com/carlos7ags/folio/html, the layout engine in .../layout, and
// font handling in .../font. Reach for those subpackages directly.
package folio

import "github.com/carlos7ags/folio/document"

// Core document types, re-exported as aliases from the document package.
type (
	// Document is a PDF document under construction. See [document.Document].
	Document = document.Document
	// PageSize is a page's dimensions in points. See [document.PageSize].
	PageSize = document.PageSize
	// Info holds document metadata written to the /Info dictionary.
	// See [document.Info].
	Info = document.Info
	// WriteOptions controls optional writer behavior. See [document.WriteOptions].
	WriteOptions = document.WriteOptions
	// PdfAConfig configures PDF/A conformance. See [document.PdfAConfig].
	PdfAConfig = document.PdfAConfig
	// EncryptionConfig configures document encryption. See [document.EncryptionConfig].
	EncryptionConfig = document.EncryptionConfig
)

// NewDocument creates a new PDF document with the given page size.
// It is [document.NewDocument].
var NewDocument = document.NewDocument

// Standard page sizes, re-exported from the document package.
var (
	PageSizeA0 = document.PageSizeA0
	PageSizeA1 = document.PageSizeA1
	PageSizeA2 = document.PageSizeA2
	PageSizeA3 = document.PageSizeA3
	PageSizeA4 = document.PageSizeA4
	PageSizeA5 = document.PageSizeA5
	PageSizeA6 = document.PageSizeA6

	PageSizeB4 = document.PageSizeB4
	PageSizeB5 = document.PageSizeB5

	PageSizeLetter    = document.PageSizeLetter
	PageSizeLegal     = document.PageSizeLegal
	PageSizeTabloid   = document.PageSizeTabloid
	PageSizeLedger    = document.PageSizeLedger
	PageSizeExecutive = document.PageSizeExecutive
)
