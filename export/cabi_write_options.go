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
	"fmt"

	"github.com/carlos7ags/folio/document"
)

// folio_write_options_new allocates a zero-valued WriteOptions handle.
// The zero value reproduces the historical default writer output; set
// individual toggles before passing the handle to
// [folio_document_save_with_options] or
// [folio_document_write_to_buffer_with_options].
//
//export folio_write_options_new
func folio_write_options_new() C.uint64_t {
	opts := &document.WriteOptions{}
	return C.uint64_t(ht.store(opts))
}

// folio_write_options_free removes a WriteOptions handle from the
// handle table. The same handle may be reused across multiple writer
// calls before being freed.
//
//export folio_write_options_free
func folio_write_options_free(optsH C.uint64_t) {
	ht.delete(uint64(optsH))
}

// folio_write_options_set_use_xref_stream enables or disables the
// cross-reference stream output (ISO 32000-1 §7.5.8).
//
//export folio_write_options_set_use_xref_stream
func folio_write_options_set_use_xref_stream(optsH C.uint64_t, enabled C.int32_t) C.int32_t {
	opts, errCode := loadWriteOptions(optsH)
	if errCode != errOK {
		return errCode
	}
	opts.UseXRefStream = enabled != 0
	return errOK
}

// folio_write_options_set_use_object_streams enables packing eligible
// indirect objects into compressed object streams (ISO 32000-1 §7.5.7).
// Implies UseXRefStream; the writer returns an error if this is set
// without UseXRefStream.
//
//export folio_write_options_set_use_object_streams
func folio_write_options_set_use_object_streams(optsH C.uint64_t, enabled C.int32_t) C.int32_t {
	opts, errCode := loadWriteOptions(optsH)
	if errCode != errOK {
		return errCode
	}
	opts.UseObjectStreams = enabled != 0
	return errOK
}

// folio_write_options_set_object_stream_capacity caps the number of
// objects packed per /ObjStm. Zero means "use the writer default"
// (100). Ignored unless UseObjectStreams is enabled.
//
//export folio_write_options_set_object_stream_capacity
func folio_write_options_set_object_stream_capacity(optsH C.uint64_t, capacity C.int32_t) C.int32_t {
	opts, errCode := loadWriteOptions(optsH)
	if errCode != errOK {
		return errCode
	}
	opts.ObjectStreamCapacity = int(capacity)
	return errOK
}

// folio_write_options_set_orphan_sweep enables the orphan-sweep pass
// that drops indirect objects unreachable from /Root, /Info, and
// /Encrypt and renumbers survivors contiguously.
//
//export folio_write_options_set_orphan_sweep
func folio_write_options_set_orphan_sweep(optsH C.uint64_t, enabled C.int32_t) C.int32_t {
	opts, errCode := loadWriteOptions(optsH)
	if errCode != errOK {
		return errCode
	}
	opts.OrphanSweep = enabled != 0
	return errOK
}

// folio_write_options_set_clean_content_streams enables removing empty
// q...Q save/restore pairs and identity 1 0 0 1 0 0 cm operators from
// page content streams (ISO 32000-1 §7.8).
//
//export folio_write_options_set_clean_content_streams
func folio_write_options_set_clean_content_streams(optsH C.uint64_t, enabled C.int32_t) C.int32_t {
	opts, errCode := loadWriteOptions(optsH)
	if errCode != errOK {
		return errCode
	}
	opts.CleanContentStreams = enabled != 0
	return errOK
}

// folio_write_options_set_deduplicate_objects enables merging of
// byte-identical indirect objects. Surviving objects are renumbered
// contiguously; references to merged duplicates are rewritten to the
// canonical survivor.
//
//export folio_write_options_set_deduplicate_objects
func folio_write_options_set_deduplicate_objects(optsH C.uint64_t, enabled C.int32_t) C.int32_t {
	opts, errCode := loadWriteOptions(optsH)
	if errCode != errOK {
		return errCode
	}
	opts.DeduplicateObjects = enabled != 0
	return errOK
}

// folio_write_options_set_recompress_streams enables re-Flate
// compression of eligible stream payloads at zlib.BestCompression,
// gated by a size-regression guard that reverts any rewrite failing
// to produce a strictly smaller payload.
//
//export folio_write_options_set_recompress_streams
func folio_write_options_set_recompress_streams(optsH C.uint64_t, enabled C.int32_t) C.int32_t {
	opts, errCode := loadWriteOptions(optsH)
	if errCode != errOK {
		return errCode
	}
	opts.RecompressStreams = enabled != 0
	return errOK
}

// folio_document_save_with_options writes the document to a PDF file
// at path using the supplied WriteOptions. The options handle may be
// zero to use the historical defaults, equivalent to
// [folio_document_save].
//
//export folio_document_save_with_options
func folio_document_save_with_options(docH C.uint64_t, path *C.char, optsH C.uint64_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	opts, errCode := loadWriteOptionsOrDefault(optsH)
	if errCode != errOK {
		return errCode
	}
	if err := doc.SaveWithOptions(C.GoString(path), opts); err != nil {
		return setErr(errIO, err)
	}
	return errOK
}

// folio_document_write_to_buffer_with_options renders the document to
// an in-memory buffer using the supplied WriteOptions and returns the
// buffer handle. The options handle may be zero to use the historical
// defaults, equivalent to [folio_document_write_to_buffer]. Returns 0
// on error; call [folio_last_error] for details.
//
//export folio_document_write_to_buffer_with_options
func folio_document_write_to_buffer_with_options(docH C.uint64_t, optsH C.uint64_t) C.uint64_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return 0
	}
	opts, errCode := loadWriteOptionsOrDefault(optsH)
	if errCode != errOK {
		return 0
	}
	var buf bytes.Buffer
	if _, err := doc.WriteToWithOptions(&buf, opts); err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(newCBuffer(buf.Bytes())))
}

// loadWriteOptions retrieves a *document.WriteOptions from the handle
// table. Unlike loadWriteOptionsOrDefault, this returns an error for
// a zero handle — setters cannot mutate a non-existent options object.
func loadWriteOptions(h C.uint64_t) (*document.WriteOptions, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid write options handle")
		return nil, errInvalidHandle
	}
	opts, ok := v.(*document.WriteOptions)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a WriteOptions (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return opts, errOK
}

// loadWriteOptionsOrDefault retrieves WriteOptions from the handle
// table, returning a zero-value struct when the handle is zero. The
// writer-facing calls accept a zero handle as "use defaults" for
// ergonomics — callers that only want the optimizer pass for one
// write do not need to allocate and free an options object.
func loadWriteOptionsOrDefault(h C.uint64_t) (document.WriteOptions, C.int32_t) {
	if h == 0 {
		return document.WriteOptions{}, errOK
	}
	opts, errCode := loadWriteOptions(h)
	if errCode != errOK {
		return document.WriteOptions{}, errCode
	}
	return *opts, errOK
}
