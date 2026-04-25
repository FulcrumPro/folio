// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build js

package html

import (
	"fmt"
	"net/http"

	folioimage "github.com/carlos7ags/folio/image"
)

// makeCSSFetcher is a stub for WASM builds — returns nil (no HTTP fetching).
func makeCSSFetcher(_ URLPolicy, _ *http.Client) func(string) ([]byte, error) {
	return nil
}

// fetchImage is a stub for WASM builds where net/http is not available.
// HTTP/HTTPS image URLs cannot be fetched in the browser. Use base64
// data URIs instead: <img src="data:image/png;base64,...">
func (c *converter) fetchImage(url string) (*folioimage.Image, error) {
	return nil, fmt.Errorf("HTTP image URLs not supported in WASM (use data: URIs instead): %s", url)
}

// httpGetBytes is a stub for WASM builds — net/http is not usable in browser.
func httpGetBytes(_ *http.Client, url string, _ int64) ([]byte, error) {
	return nil, fmt.Errorf("HTTP fetch not supported in WASM: %s", url)
}
