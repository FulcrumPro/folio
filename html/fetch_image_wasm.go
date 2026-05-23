// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build js

package html

import (
	"fmt"
	"net/http"
)

// httpGetBytes is a stub for WASM builds — net/http is not usable in browser.
// Asset references that would otherwise produce a remote fetch surface this
// error through the same channel as any other resolveLocalAsset failure.
// Use base64 data URIs (`<img src="data:image/png;base64,...">`) for inline
// assets in browser builds.
func httpGetBytes(_ *http.Client, url string, _ int64) ([]byte, error) {
	return nil, fmt.Errorf("html: HTTP fetch not supported in WASM: %s", url)
}
