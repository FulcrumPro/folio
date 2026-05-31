// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

// TestParseErrorTypeContract pins the contract callers rely on: a
// *ParseError carries its cause, Unwraps to it, and is discoverable with
// errors.As. The html parser is lenient enough that triggering a real parse
// failure from a string is impractical, so the type contract is exercised
// directly.
func TestParseErrorTypeContract(t *testing.T) {
	cause := errors.New("boom")
	var err error = &ParseError{Err: cause}

	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatal("errors.As should recover *ParseError")
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
	if pe.Error() != "folio/html: parse: boom" {
		t.Errorf("unexpected message: %q", pe.Error())
	}
}

// TestAssetErrorTypedFromStrictAssets verifies that a strict-mode asset
// failure surfaces as a *AssetError with a usable Category and Ref, and
// that the underlying cause is still reachable with errors.Is — the signal
// a status-mapping caller needs to classify the failure.
func TestAssetErrorTypedFromStrictAssets(t *testing.T) {
	src := `<html><head><style>
		@font-face { font-family: 'X'; src: url('missing.ttf'); }
	</style></head><body><p>hi</p></body></html>`
	_, err := Convert(src, &Options{BaseFS: fstest.MapFS{}, StrictAssets: true})
	if err == nil {
		t.Fatal("expected a strict-mode asset error")
	}

	var ae *AssetError
	if !errors.As(err, &ae) {
		t.Fatalf("errors.As should recover *AssetError, got %T: %v", err, err)
	}
	if ae.Category != "@font-face" {
		t.Errorf("Category = %q, want %q", ae.Category, "@font-face")
	}
	if ae.Ref != "missing.ttf" {
		t.Errorf("Ref = %q, want %q", ae.Ref, "missing.ttf")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Error("underlying fs.ErrNotExist should remain reachable via errors.Is")
	}
}

// TestAssetErrorImageCategoryAndRef checks the Category/Ref extraction for
// an image reference, whose attrs lead with the "src" key.
func TestAssetErrorImageCategoryAndRef(t *testing.T) {
	src := `<html><body><img src="gone.png"></body></html>`
	_, err := Convert(src, &Options{BaseFS: fstest.MapFS{}, StrictAssets: true})
	if err == nil {
		t.Fatal("expected a strict-mode asset error")
	}

	var ae *AssetError
	if !errors.As(err, &ae) {
		t.Fatalf("errors.As should recover *AssetError, got %T: %v", err, err)
	}
	if ae.Category != "image" {
		t.Errorf("Category = %q, want %q", ae.Category, "image")
	}
	if ae.Ref != "gone.png" {
		t.Errorf("Ref = %q, want %q", ae.Ref, "gone.png")
	}
}

// TestAssetErrorRefSkipsFontFaceFamily guards the Ref-extraction rule that
// @font-face attrs lead with "family" but the meaningful reference is the
// "src" URL, not the family name.
func TestAssetErrorRefSkipsFontFaceFamily(t *testing.T) {
	cause := errors.New("nope")
	err := formatAssetError("@font-face", cause, []any{"family", "MyFamily", "src", "f.woff2", "origin", "fs"})

	var ae *AssetError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AssetError, got %T", err)
	}
	if ae.Ref != "f.woff2" {
		t.Errorf("Ref = %q, want the src value %q (not the family)", ae.Ref, "f.woff2")
	}
}
