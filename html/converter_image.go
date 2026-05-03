// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/svg"

	"golang.org/x/net/html"
)

// convertImage handles <img> elements.
func (c *converter) convertImage(n *html.Node, style computedStyle) []layout.Element {
	src := getAttr(n, "src")
	alt := getAttr(n, "alt")

	if src == "" {
		if alt != "" {
			return c.altTextFallback(alt, style)
		}
		return nil
	}

	// Check if src references an SVG file — route to SVG converter.
	if isSVGSource(src) {
		return c.convertImgSVG(src, alt, style)
	}

	// Load image: data URI, HTTP URL, or local path.
	var img *folioimage.Image
	var err error

	// prevent loading if interceptor returned an image or an error
	if strings.HasPrefix(src, "data:") {
		img, err = decodeDataURI(src)
	} else {
		img, err = c.loadImageAsset(src)
	}
	if err != nil {
		c.reportAssetError("image", err, "src", src)
		if alt != "" {
			return c.altTextFallback(alt, style)
		}
		return c.altTextFallback("[image: "+src+"]", style)
	}

	ie := layout.NewImageElement(img)

	// Parse width/height from attributes or CSS.
	w := parseAttrFloat(getAttr(n, "width"))
	h := parseAttrFloat(getAttr(n, "height"))
	if style.Width != nil {
		w = style.Width.toPoints(0, style.FontSize)
	}
	if style.Height != nil {
		h = style.Height.toPoints(0, style.FontSize)
	}
	if w > 0 || h > 0 {
		ie.SetSize(w, h)
	}
	// CSS max-width / max-height / min-width / min-height apply to
	// replaced elements like <img> per CSS 2.1 §10.7. Pre-fix the
	// converter only consulted style.Width/Height, so a high-res
	// source image with `max-width: 180px` ignored the cap and bled
	// the full container width. Threading these into ImageElement
	// lets layout/image.go::resolveSize clamp post-resolution while
	// preserving aspect ratio.
	var maxW, maxH float64
	if style.MaxWidth != nil {
		maxW = style.MaxWidth.toPoints(0, style.FontSize)
	}
	if style.MaxHeight != nil {
		maxH = style.MaxHeight.toPoints(0, style.FontSize)
	}
	if maxW > 0 || maxH > 0 {
		ie.SetMaxSize(maxW, maxH)
	}
	var minW, minH float64
	if style.MinWidth != nil {
		minW = style.MinWidth.toPoints(0, style.FontSize)
	}
	if style.MinHeight != nil {
		minH = style.MinHeight.toPoints(0, style.FontSize)
	}
	if minW > 0 || minH > 0 {
		ie.SetMinSize(minW, minH)
	}
	if style.ObjectFit != "" {
		ie.SetObjectFit(style.ObjectFit)
	}
	if style.ObjectPosition != "" {
		ie.SetObjectPosition(style.ObjectPosition)
	}

	return []layout.Element{ie}
}

// convertSVG handles inline <svg> elements.
func (c *converter) convertSVG(n *html.Node, style computedStyle) []layout.Element {
	// Serialize the <svg> HTML node back to markup so the SVG parser can read it.
	var buf bytes.Buffer
	if err := html.Render(&buf, n); err != nil {
		return nil
	}

	s, err := svg.Parse(buf.String())
	if err != nil {
		return nil // skip invalid SVG
	}

	el := layout.NewSVGElement(s)

	// Apply explicit size from CSS or SVG attributes.
	w := s.Width()
	h := s.Height()
	if style.Width != nil {
		w = style.Width.toPoints(0, style.FontSize)
	}
	if style.Height != nil {
		h = style.Height.toPoints(0, style.FontSize)
	}
	if w > 0 || h > 0 {
		el.SetSize(w, h)
	}

	return []layout.Element{el}
}

// altTextFallback returns a paragraph with alt text when an image can't be loaded.
func (c *converter) altTextFallback(alt string, style computedStyle) []layout.Element {
	stdFont, embFont := c.resolveFontPair(style)
	var p *layout.Paragraph
	if embFont != nil {
		p = layout.NewParagraphEmbedded(alt, embFont, style.FontSize)
	} else {
		p = layout.NewParagraph(alt, stdFont, style.FontSize)
	}
	return []layout.Element{p}
}

// decodeDataURI parses a data: URI and returns the image.
// Format: data:[<mediatype>][;base64],<data>
func decodeDataURI(uri string) (*folioimage.Image, error) {
	// Strip "data:" prefix.
	rest := strings.TrimPrefix(uri, "data:")

	// Split at comma: metadata,data
	commaIdx := strings.IndexByte(rest, ',')
	if commaIdx < 0 {
		return nil, fmt.Errorf("invalid data URI: no comma")
	}
	meta := rest[:commaIdx]
	encoded := rest[commaIdx+1:]

	// Decode data.
	var data []byte
	if strings.Contains(meta, ";base64") {
		var err error
		data, err = base64Decode(encoded)
		if err != nil {
			return nil, fmt.Errorf("data URI base64: %w", err)
		}
	} else {
		data = []byte(encoded)
	}

	// Detect format from media type.
	if strings.Contains(meta, "image/jpeg") || strings.Contains(meta, "image/jpg") {
		return folioimage.NewJPEG(data)
	}
	if strings.Contains(meta, "image/png") {
		return folioimage.NewPNG(data)
	}
	if strings.Contains(meta, "image/webp") {
		return folioimage.NewWebP(data)
	}
	if strings.Contains(meta, "image/gif") {
		return folioimage.NewGIF(data)
	}

	// Fallback: content sniffing by magic bytes.
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return folioimage.NewJPEG(data)
	}
	if len(data) >= 4 && string(data[:4]) == "\x89PNG" {
		return folioimage.NewPNG(data)
	}
	if len(data) >= 4 && string(data[:4]) == "RIFF" && len(data) >= 12 && string(data[8:12]) == "WEBP" {
		return folioimage.NewWebP(data)
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return folioimage.NewGIF(data)
	}
	if img, err := folioimage.NewJPEG(data); err == nil {
		return img, nil
	}
	return folioimage.NewPNG(data)
}

// base64Decode decodes standard base64.
func base64Decode(s string) ([]byte, error) {
	// Remove whitespace (common in data URIs).
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, s)

	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var lookup [256]byte
	for i := range lookup {
		lookup[i] = 0xFF
	}
	for i, c := range alphabet {
		lookup[c] = byte(i)
	}

	// Estimate output size.
	out := make([]byte, 0, len(s)*3/4)
	var buf uint32
	var bits int

	for _, c := range []byte(s) {
		if c == '=' {
			break
		}
		val := lookup[c]
		if val == 0xFF {
			continue // skip unknown chars
		}
		buf = buf<<6 | uint32(val)
		bits += 6
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(buf>>bits))
			buf &= (1 << bits) - 1
		}
	}

	return out, nil
}

// isSVGSource checks if a source string references an SVG file.
func isSVGSource(src string) bool {
	if strings.HasPrefix(src, "data:image/svg") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(strings.SplitN(src, "?", 2)[0]))
	return ext == ".svg"
}

// convertImgSVG loads an SVG file referenced by <img src> and returns it as a
// layout element. On any failure it logs a warning and falls back to the alt
// text (or a "[image: src]" placeholder when alt is empty), matching the
// raster <img> failure path.
func (c *converter) convertImgSVG(src, alt string, style computedStyle) []layout.Element {
	var svgData []byte
	var err error

	if strings.HasPrefix(src, "data:") {
		rest := strings.TrimPrefix(src, "data:")
		commaIdx := strings.IndexByte(rest, ',')
		if commaIdx < 0 {
			err = fmt.Errorf("invalid data URI: no comma")
		} else {
			meta := rest[:commaIdx]
			encoded := rest[commaIdx+1:]
			if strings.Contains(meta, ";base64") {
				svgData, err = base64Decode(encoded)
			} else {
				svgData = []byte(encoded)
			}
		}
	} else {
		svgData, err = c.resolveLocalAsset("", src, 50<<20)
	}

	if err == nil {
		var s *svg.SVG
		s, err = svg.Parse(string(svgData))
		if err == nil {
			el := layout.NewSVGElement(s)
			w := s.Width()
			h := s.Height()
			if style.Width != nil {
				w = style.Width.toPoints(0, style.FontSize)
			}
			if style.Height != nil {
				h = style.Height.toPoints(0, style.FontSize)
			}
			if w > 0 || h > 0 {
				el.SetSize(w, h)
			}
			return []layout.Element{el}
		}
	}

	c.reportAssetError("SVG image", err, "src", src)
	if alt != "" {
		return c.altTextFallback(alt, style)
	}
	return c.altTextFallback("[image: "+src+"]", style)
}

// isURL checks if a string is an HTTP(S) URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// loadImageAsset routes a non-data-URI image src through the unified
// [resolveLocalAsset] contract: http(s) fetch via Options.Client,
// absolute filesystem path with BaseFS nil, or BaseFS-relative path.
// The image format is detected from the file extension first, then by
// magic bytes.
func (c *converter) loadImageAsset(src string) (*folioimage.Image, error) {
	data, err := c.resolveLocalAsset("", src, 50<<20)
	if err != nil {
		return nil, err
	}
	return decodeImageBytes(data, filepath.Ext(src))
}

// decodeImageBytes decodes image bytes to a folio Image. ext is a hint like
// ".png" / ".jpg"; content sniffing is used when the extension is unknown.
func decodeImageBytes(data []byte, ext string) (*folioimage.Image, error) {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return folioimage.NewJPEG(data)
	case ".png":
		return folioimage.NewPNG(data)
	case ".tif", ".tiff":
		return folioimage.NewTIFF(data)
	case ".webp":
		return folioimage.NewWebP(data)
	case ".gif":
		return folioimage.NewGIF(data)
	}
	// Content sniffing: try formats by magic bytes.
	if len(data) >= 4 && string(data[:4]) == "RIFF" && len(data) >= 12 && string(data[8:12]) == "WEBP" {
		return folioimage.NewWebP(data)
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return folioimage.NewGIF(data)
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return folioimage.NewJPEG(data)
	}
	if len(data) >= 4 && string(data[:4]) == "\x89PNG" {
		return folioimage.NewPNG(data)
	}
	if img, err := folioimage.NewJPEG(data); err == nil {
		return img, nil
	}
	return folioimage.NewPNG(data)
}
