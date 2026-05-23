// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package html

import (
	"fmt"
	"io"
	"net/http"
)

// httpGetBytes performs a GET with the supplied client and returns at most
// maxBytes of body. Non-200 responses surface as an error. Centralized HTTP
// fetching for every asset type goes through here via [fetchHTTPBytes];
// the helper is split out so the WASM build can stub it.
func httpGetBytes(client *http.Client, url string, maxBytes int64) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("html: fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("html: fetch %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}
