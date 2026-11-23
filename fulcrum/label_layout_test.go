// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package fulcrum

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
)

// The tests in this file pin the v0.9.1-fulcrum.39 patch, found by rendering
// the .NET DocGen customizable job label (CustomizableLabel.hbs) through
// folio. The label floats its logo and QR code right (`.right { float: right;
// text-align: right }`) beside the job rows, inside a fixed-size
// `.printlabel { display: flex; flex-direction: column; overflow: hidden }`.
//
// Every test renders on US Letter with folio's default 72pt margins, so a
// 400pt container spans x = 72..472 and its top edge is at y = 720.

const (
	containerLeft  = 72.0
	containerRight = 472.0
	contentTop     = 720.0
)

// TestAutoWidthFloatShrinksToContent: a float with `width: auto` is sized
// shrink-to-fit (CSS 2.1 §10.3.5). folio laid the float's content out at the
// full container width, so a float holding a block wrapper took the whole line:
// the label's right-floated QR code drew at the left edge, on top of the rows.
// The wrapper has a margin, as the label's containers do: the converter drops
// a wrapper with no box styles, and a float holding just the image already fit.
func TestAutoWidthFloatShrinksToContent(t *testing.T) {
	src := `<html><head><style>body{margin:0;padding:0}</style></head><body>
		<div style="width:400pt">
			<div style="float:right"><div style="margin-bottom:2pt"><img src="` + pngDataURI(200, 100) + `" style="width:100pt"></div></div>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	x, w, ok := largestImageXW(pdf)
	if !ok {
		t.Fatal("image not drawn")
	}
	if !near(w, 100) || !near(x+w, containerRight) {
		t.Errorf("image x=%.1f w=%.1f right=%.1f, want w=100 against the right edge %.0f", x, w, x+w, containerRight)
	}
}

// TestFloatedImageMaxWidthSizesFloat: an image capped by CSS max-width
// measures at that cap, so a shrink-to-fit float holding it is only as wide
// as the image draws. folio measured the image at its natural width, so the
// label's logo (`max-width: 50px` on a larger image) made the float wider
// than its content and pushed the logo off the right edge.
func TestFloatedImageMaxWidthSizesFloat(t *testing.T) {
	src := `<html><head><style>body{margin:0;padding:0}</style></head><body>
		<div style="width:400pt">
			<div style="float:right"><div style="margin-bottom:2pt"><img src="` + pngDataURI(300, 150) + `" style="max-width:80pt"></div></div>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	x, w, ok := largestImageXW(pdf)
	if !ok {
		t.Fatal("image not drawn")
	}
	if !near(w, 80) || !near(x+w, containerRight) {
		t.Errorf("image x=%.1f w=%.1f right=%.1f, want w=80 against the right edge %.0f", x, w, x+w, containerRight)
	}
}

// TestTextBesideFloatWraps: content that follows a float inside a box lays
// out in the width beside the float (CSS 2.1 §9.5). folio narrowed content
// beside a float only at the page level, so inside a box the label rows ran
// underneath the QR code.
func TestTextBesideFloatWraps(t *testing.T) {
	src := `<html><head><style>body{margin:0;padding:0;font-size:10pt}</style></head><body>
		<div style="width:400pt">
			<div style="float:right;width:150pt;height:100pt;background:#cccccc"></div>
			<div>Mounting bracket three sixteenths A36 powder coat black, press brake then weld, quantity twenty four of one hundred twenty, due December fourth.</div>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	floatLeft := containerRight - 150
	floatBottom := contentTop - 100
	beside := 0
	for _, s := range pageSpans(t, pdf) {
		if s.Y <= floatBottom || s.Text == "" {
			continue
		}
		beside++
		if right := s.X + s.Width; right > floatLeft+0.5 {
			t.Errorf("span %q ends at x=%.1f, under the float that starts at x=%.0f", s.Text, right, floatLeft)
		}
	}
	if beside == 0 {
		t.Fatal("no text drawn beside the float")
	}
}

// TestFixedHeightColumnKeepsOverflow: a definite-height column flex whose
// items are taller than the box draws them overflowing the box (clipped by
// overflow:hidden), as a browser does, and the box keeps its height. folio
// reported that nothing fit, even on an empty page, so the 2in x 1in job label
// rendered as a blank page.
func TestFixedHeightColumnKeepsOverflow(t *testing.T) {
	src := `<html><head><style>body{margin:0;padding:0;font-size:10pt}</style></head><body>
		<div style="display:flex;flex-direction:column;width:200pt;height:40pt;overflow:hidden">
			<div style="flex-grow:1">FIRSTROW<br>two<br>three<br>four<br>five</div>
			<div>FOOTERROW</div>
		</div>
		<div>AFTERBOX</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if n := pageCount(pdf); n != 1 {
		t.Errorf("page count = %d, want 1", n)
	}
	if y := findTextY(pdf, "FIRSTROW"); y < contentTop-40 {
		t.Errorf("FIRSTROW y=%.1f, want it drawn inside the 40pt box below y=%.0f", y, contentTop)
	}
	after := findTextY(pdf, "AFTERBOX")
	if after < 0 || after > contentTop-40 || after < contentTop-40-20 {
		t.Errorf("AFTERBOX y=%.1f, want the line just below the 40pt box (y=%.0f)", after, contentTop-40)
	}
}

// TestImageFollowsTextAlign: an <img> is inline content, so it sits where the
// containing block's text-align puts its line (CSS 2.1 §16.2). folio drew
// every image at the left edge, so the label's right-aligned logo sat at the
// left of its float.
func TestImageFollowsTextAlign(t *testing.T) {
	for _, tc := range []struct {
		align string
		wantX float64
	}{
		{"left", containerLeft},
		{"center", containerLeft + 150},
		{"right", containerRight - 100},
	} {
		src := `<html><head><style>body{margin:0;padding:0}</style></head><body>
			<div style="width:400pt;text-align:` + tc.align + `"><img src="` + pngDataURI(200, 100) + `" style="width:100pt"></div>
		</body></html>`
		pdf, err := renderHTMLToPDF(src)
		if err != nil {
			t.Fatalf("%s: render: %v", tc.align, err)
		}
		x, _, ok := largestImageXW(pdf)
		if !ok {
			t.Fatalf("%s: image not drawn", tc.align)
		}
		if !near(x, tc.wantX) {
			t.Errorf("text-align:%s image x=%.1f, want %.0f", tc.align, x, tc.wantX)
		}
	}
}

// TestFloatedImageIgnoresTextAlign: text-align moves inline content only. A
// floated <img> under text-align:right must stay as wide as the image, so the
// text beside it keeps the rest of the line.
func TestFloatedImageIgnoresTextAlign(t *testing.T) {
	src := `<html><head><style>body{margin:0;padding:0;font-size:10pt}</style></head><body>
		<div style="width:400pt;text-align:right">
			<img src="` + pngDataURI(200, 100) + `" style="float:right;width:60pt">
			<div>Beside the floated image</div>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	x, w, ok := largestImageXW(pdf)
	if !ok {
		t.Fatal("image not drawn")
	}
	if !near(x+w, containerRight) {
		t.Errorf("image right=%.1f, want %.0f", x+w, containerRight)
	}
	beside := 0
	for _, s := range pageSpans(t, pdf) {
		if s.Text == "" || s.Y < contentTop-30 {
			continue
		}
		beside++
		if s.X+s.Width > x+0.5 {
			t.Errorf("span %q ends at x=%.1f, under the image at x=%.1f", s.Text, s.X+s.Width, x)
		}
	}
	if beside == 0 {
		t.Fatal("no text drawn beside the image")
	}
}

// TestClearInsideBoxMovesBelowFloat: a box with clear:both after a float in
// the same box starts below the float, not beside it.
func TestClearInsideBoxMovesBelowFloat(t *testing.T) {
	src := `<html><head><style>body{margin:0;padding:0;font-size:10pt}</style></head><body>
		<div style="width:400pt">
			<div style="float:right;width:150pt;height:100pt;background:#cccccc"></div>
			<div style="clear:both">CLEARED</div>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if y := findTextY(pdf, "CLEARED"); y < 0 || y > contentTop-100 {
		t.Errorf("CLEARED y=%.1f, want below the float bottom y=%.0f", y, contentTop-100)
	}
}

// TestFloatHoldingTableKeepsWidth: a table reports no intrinsic width, so a
// float holding one is not shrunk to fit (it would crush the table); it keeps
// the full-width layout it had before.
func TestFloatHoldingTableKeepsWidth(t *testing.T) {
	src := `<html><head><style>body{margin:0;padding:0;font-size:10pt}</style></head><body>
		<div style="width:400pt">
			<div style="float:right"><div style="padding:4pt">
				<table><tr><td>Subtotal</td><td>1,250.00</td></tr></table>
			</div></div>
		</div>
	</body></html>`
	pdf, err := renderHTMLToPDF(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	label, amount := findTextY(pdf, "Subtotal"), findTextY(pdf, "1,250.00")
	if label < 0 || amount < 0 || !near(label, amount) {
		t.Errorf("Subtotal y=%.1f, amount y=%.1f, want one row on one line", label, amount)
	}
}

// pngDataURI returns a solid w x h PNG as a data URI.
func pngDataURI(w, h int) string {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{0x1f, 0x4e, 0x79, 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func near(got, want float64) bool { return math.Abs(got-want) <= 1 }
