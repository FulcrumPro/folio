// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"io"
	"runtime"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// The tests encode every fixture with the forward transform, then make
// sure that applyPredictor gives back the original bytes. The encoders
// below follow PNG 1.2 §6 and TIFF 6.0 §14 directly and share no code
// with the decoder.

// decodeParms builds a /DecodeParms dictionary. A value of 0 leaves the
// key out, so that the decoder uses its default.
func decodeParms(predictor, colors, bpc, columns int) *core.PdfDictionary {
	d := core.NewPdfDictionary()
	d.Set("Predictor", core.NewPdfInteger(predictor))
	if colors != 0 {
		d.Set("Colors", core.NewPdfInteger(colors))
	}
	if bpc != 0 {
		d.Set("BitsPerComponent", core.NewPdfInteger(bpc))
	}
	if columns != 0 {
		d.Set("Columns", core.NewPdfInteger(columns))
	}
	return d
}

// pngFilterRows applies PNG filter filters[r] to row r of raw and
// prefixes each row with its filter type byte.
func pngFilterRows(raw []byte, rowLen, bpp int, filters []byte) []byte {
	var out []byte
	prev := make([]byte, rowLen)
	for r := 0; r*rowLen < len(raw); r++ {
		cur := raw[r*rowLen : (r+1)*rowLen]
		ft := filters[r%len(filters)]
		out = append(out, ft)
		for i := range rowLen {
			var a, c byte
			if i >= bpp {
				a, c = cur[i-bpp], prev[i-bpp]
			}
			b := prev[i]
			var pred byte
			switch ft {
			case 1:
				pred = a
			case 2:
				pred = b
			case 3:
				pred = byte((int(a) + int(b)) / 2)
			case 4:
				pred = refPaeth(a, b, c)
			}
			out = append(out, cur[i]-pred)
		}
		prev = cur
	}
	return out
}

// refPaeth is the Paeth predictor as PNG 1.2 §6.6 writes it.
func refPaeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa, pb, pc := p-int(a), p-int(b), p-int(c)
	if pa < 0 {
		pa = -pa
	}
	if pb < 0 {
		pb = -pb
	}
	if pc < 0 {
		pc = -pc
	}
	switch {
	case pa <= pb && pa <= pc:
		return a
	case pb <= pc:
		return b
	}
	return c
}

// getSample and setSample access sample i of a row packed at bpc bits
// per sample, high-order bits first.
func getSample(row []byte, i, bpc int) int {
	switch bpc {
	case 8:
		return int(row[i])
	case 16:
		return int(binary.BigEndian.Uint16(row[2*i:]))
	}
	bit := i * bpc
	return int(row[bit/8]>>(8-bit%8-bpc)) & (1<<bpc - 1)
}

func setSample(row []byte, i, bpc, v int) {
	switch bpc {
	case 8:
		row[i] = byte(v)
		return
	case 16:
		binary.BigEndian.PutUint16(row[2*i:], uint16(v))
		return
	}
	bit := i * bpc
	shift := 8 - bit%8 - bpc
	mask := byte(1<<bpc-1) << shift
	row[bit/8] = row[bit/8]&^mask | byte(v<<shift)&mask
}

// tiffDiffRows applies TIFF horizontal differencing to each row of raw.
func tiffDiffRows(raw []byte, rowLen, colors, bpc, samples int) []byte {
	out := bytes.Clone(raw)
	mod := 1 << bpc
	for r := 0; r*rowLen < len(raw); r++ {
		src := raw[r*rowLen : (r+1)*rowLen]
		dst := out[r*rowLen : (r+1)*rowLen]
		for i := colors; i < samples; i++ {
			d := getSample(src, i, bpc) - getSample(src, i-colors, bpc)
			setSample(dst, i, bpc, (d%mod+mod)%mod)
		}
	}
	return out
}

// pattern returns n bytes that change in every position, so that each
// filter and each neighbor distance gives a different result.
func pattern(n int, seed uint32) []byte {
	out := make([]byte, n)
	x := seed
	for i := range out {
		x = x*1664525 + 1013904223
		out[i] = byte(x >> 24)
	}
	return out
}

// maskPadding sets the unused low-order bits at the end of each row to
// zero, so that the fixture has the form that a PDF producer writes.
func maskPadding(raw []byte, rowLen, bitsPerRow int) {
	pad := rowLen*8 - bitsPerRow
	if pad == 0 {
		return
	}
	for r := 0; r*rowLen < len(raw); r++ {
		raw[(r+1)*rowLen-1] &^= byte(1<<pad - 1)
	}
}

func mustDecode(t *testing.T, data []byte, parms *core.PdfDictionary) []byte {
	t.Helper()
	got, err := applyPredictor(data, parms)
	if err != nil {
		t.Fatalf("applyPredictor: %v", err)
	}
	return got
}

// TestPNGPredictorRGB8EachFilter covers each PNG filter type with
// 3-color 8-bit data, where the left neighbor is 3 bytes back.
func TestPNGPredictorRGB8EachFilter(t *testing.T) {
	const columns, rows = 5, 4
	rowLen := columns * 3
	raw := pattern(rowLen*rows, 7)
	cases := []struct {
		name    string
		filters []byte
	}{
		{"None", []byte{0}},
		{"Sub", []byte{1}},
		{"Up", []byte{2}},
		{"Average", []byte{3}},
		{"Paeth", []byte{4}},
		{"Mixed", []byte{4, 1, 3, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := pngFilterRows(raw, rowLen, 3, tc.filters)
			for _, predictor := range []int{10, 15} {
				got := mustDecode(t, enc, decodeParms(predictor, 3, 8, columns))
				if !bytes.Equal(got, raw) {
					t.Fatalf("Predictor %d:\n got % x\nwant % x", predictor, got, raw)
				}
			}
		})
	}
}

// TestPNGPredictorSubByteGray covers 1-, 2- and 4-bit gray. Columns is
// not a multiple of 8, so each row ends with padding bits, and the row
// length in bytes is less than Columns.
func TestPNGPredictorSubByteGray(t *testing.T) {
	const columns, rows = 13, 5
	for _, bpc := range []int{1, 2, 4} {
		rowLen := (columns*bpc + 7) / 8
		raw := pattern(rowLen*rows, uint32(bpc))
		maskPadding(raw, rowLen, columns*bpc)
		enc := pngFilterRows(raw, rowLen, 1, []byte{0, 1, 2, 3, 4})
		got := mustDecode(t, enc, decodeParms(12, 1, bpc, columns))
		if !bytes.Equal(got, raw) {
			t.Errorf("bpc %d:\n got % x\nwant % x", bpc, got, raw)
		}
	}
}

// TestPNGPredictorSixteenBit uses 16-bit RGB, where the left neighbor
// is 6 bytes back.
func TestPNGPredictorSixteenBit(t *testing.T) {
	const columns, rows = 3, 5
	rowLen := columns * 3 * 2
	raw := pattern(rowLen*rows, 16)
	enc := pngFilterRows(raw, rowLen, 6, []byte{0, 1, 2, 3, 4})
	got := mustDecode(t, enc, decodeParms(14, 3, 16, columns))
	if !bytes.Equal(got, raw) {
		t.Fatalf("\n got % x\nwant % x", got, raw)
	}
}

// idatPayload encodes img with image/png and returns the inflated IDAT
// data: the PNG-filtered rows, in the form that a PDF producer copies
// into an image stream with /Predictor 15.
func idatPayload(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()[8:] // PNG signature
	var z []byte
	for len(b) >= 12 {
		n := binary.BigEndian.Uint32(b)
		if string(b[4:8]) == "IDAT" {
			z = append(z, b[8:8+n]...)
		}
		b = b[12+n:]
	}
	r, err := zlib.NewReader(bytes.NewReader(z))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestPNGPredictorMatchesImagePNG decodes rows that Go's image/png
// encoder filtered. The encoder picks a filter for each row, so this
// fixture comes from an independent implementation.
func TestPNGPredictorMatchesImagePNG(t *testing.T) {
	const w, h = 17, 12
	rgb := image.NewNRGBA(image.Rect(0, 0, w, h))
	rgb16 := image.NewNRGBA64(image.Rect(0, 0, w, h))
	var want8, want16 []byte
	for y := range h {
		for x := range w {
			// Each row kind favours a different filter: a horizontal ramp
			// (Sub), a copy of the row above (Up), a ramp in both
			// directions (Average or Paeth), and noise.
			var v [3]uint16
			switch y % 4 {
			case 0:
				v = [3]uint16{uint16(x) * 9, uint16(x) * 5, 200 - uint16(x)*3}
			case 1:
				v = [3]uint16{uint16(x-y) * 9, uint16(x) * 5, 200 - uint16(x)*3}
				if x%2 == 0 {
					v[0] = uint16(y) * 9
				}
			case 2:
				v = [3]uint16{uint16(x+y) * 7, uint16(x+2*y) * 3, uint16(x*y) % 251}
			case 3:
				n := pattern(3, uint32(y*w+x))
				v = [3]uint16{uint16(n[0]), uint16(n[1]), uint16(n[2])}
			}
			c := color.NRGBA{byte(v[0]), byte(v[1]), byte(v[2]), 255}
			rgb.SetNRGBA(x, y, c)
			want8 = append(want8, c.R, c.G, c.B)
			c16 := color.NRGBA64{v[0] * 257, v[1]*251 + 3, v[2] * 263, 0xffff}
			rgb16.SetNRGBA64(x, y, c16)
			want16 = binary.BigEndian.AppendUint16(want16, c16.R)
			want16 = binary.BigEndian.AppendUint16(want16, c16.G)
			want16 = binary.BigEndian.AppendUint16(want16, c16.B)
		}
	}

	cases := []struct {
		name string
		img  image.Image
		bpc  int
		want []byte
	}{
		{"RGB8", rgb, 8, want8},
		{"RGB16", rgb16, 16, want16},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := idatPayload(t, tc.img)
			stride := 1 + w*3*tc.bpc/8
			if len(enc) != stride*h {
				t.Fatalf("IDAT is %d bytes, want %d rows of %d", len(enc), h, stride)
			}
			used := map[byte]bool{}
			for r := range h {
				used[enc[r*stride]] = true
			}
			t.Logf("filter types in the fixture: %v", used)
			if len(used) < 3 {
				t.Fatalf("fixture uses only filter types %v; want at least 3", used)
			}
			got := mustDecode(t, enc, decodeParms(15, 3, tc.bpc, w))
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("decoded bytes differ (filters %v)", used)
			}
		})
	}
}

// TestPNGPredictorPaethTie pins the Paeth tie order of PNG 1.2 §6.6
// when pb == pc < pa: the predictor is b (up), not c (up-left). Random
// data almost never has this tie. Here a = 10, b = 40, c = 20.
func TestPNGPredictorPaethTie(t *testing.T) {
	enc := []byte{
		0, 20, 40,
		4, 246, 5, // 246 + 20 (b) = 10; then 5 + 40 (b) = 45
	}
	want := []byte{20, 40, 10, 45}
	if got := mustDecode(t, enc, decodeParms(15, 1, 8, 2)); !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestPNGPredictorUnknownFilterType: PdfPig leaves a row with a filter
// type byte other than 0-4 as read, and the next row uses it as "up".
func TestPNGPredictorUnknownFilterType(t *testing.T) {
	enc := []byte{
		7, 10, 20, 30, 40, 50, 60,
		2, 1, 1, 1, 1, 1, 1,
		255, 9, 8, 7, 6, 5, 4,
	}
	want := []byte{
		10, 20, 30, 40, 50, 60,
		11, 21, 31, 41, 51, 61,
		9, 8, 7, 6, 5, 4,
	}
	got := mustDecode(t, enc, decodeParms(15, 3, 8, 2))
	if !bytes.Equal(got, want) {
		t.Fatalf("\n got % x\nwant % x", got, want)
	}
}

// TestPNGPredictorTruncatedRow pins the PdfPig 0.1.8 behavior for a
// short final row (PngPredictor.Decode reads into a row buffer that it
// does not clear): the row comes out full length, the missing bytes keep
// the value of the previous decoded row, and then the filter runs over
// the full row.
func TestPNGPredictorTruncatedRow(t *testing.T) {
	cases := []struct {
		name string
		enc  []byte
		want []byte
	}{
		{
			name: "Up, 4 of 6 bytes",
			enc:  []byte{0, 10, 20, 30, 40, 50, 60, 2, 1, 2, 3, 4},
			// Tail 50, 60 comes from row 0, then Up adds row 0 again.
			want: []byte{10, 20, 30, 40, 50, 60, 11, 22, 33, 44, 100, 120},
		},
		{
			name: "Sub, filter byte only",
			enc:  []byte{0, 10, 20, 30, 40, 50, 60, 1},
			// The row is all row 0, then Sub adds the pixel 3 bytes back.
			want: []byte{10, 20, 30, 40, 50, 60, 10, 20, 30, 50, 70, 90},
		},
		{
			name: "None, 2 of 6 bytes",
			enc:  []byte{0, 10, 20, 30, 40, 50, 60, 0, 7, 8},
			want: []byte{10, 20, 30, 40, 50, 60, 7, 8, 30, 40, 50, 60},
		},
		{
			name: "Average, 5 of 6 bytes",
			enc:  []byte{0, 10, 20, 30, 40, 50, 60, 3, 1, 2, 3, 4, 5},
			// Tail 60 comes from row 0. Left is 3 bytes back.
			want: []byte{10, 20, 30, 40, 50, 60, 6, 12, 18, 27, 36, 99},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustDecode(t, tc.enc, decodeParms(15, 3, 8, 2))
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("\n got %v\nwant %v", got, tc.want)
			}
		})
	}
}

// TestTIFFPredictor covers Predictor 2 at each legal bit depth, with 1
// and 3 colors.
func TestTIFFPredictor(t *testing.T) {
	const columns, rows = 11, 4
	cases := []struct{ colors, bpc int }{
		{3, 8}, {1, 8}, {3, 16}, {1, 16}, {3, 2}, {3, 4}, {1, 2}, {1, 4}, {3, 1},
	}
	for _, tc := range cases {
		bits := columns * tc.colors * tc.bpc
		rowLen := (bits + 7) / 8
		raw := pattern(rowLen*rows, uint32(tc.colors*100+tc.bpc))
		maskPadding(raw, rowLen, bits)
		enc := tiffDiffRows(raw, rowLen, tc.colors, tc.bpc, columns*tc.colors)
		if bytes.Equal(enc, raw) {
			t.Fatalf("colors %d bpc %d: fixture did not change", tc.colors, tc.bpc)
		}
		got := mustDecode(t, enc, decodeParms(2, tc.colors, tc.bpc, columns))
		if !bytes.Equal(got, raw) {
			t.Errorf("colors %d bpc %d:\n got % x\nwant % x", tc.colors, tc.bpc, got, raw)
		}
	}
}

// TestTIFFPredictorOneBitGray: 1-bit, 1-color data takes the PdfPig
// bit loop, which also runs over the padding bits at the end of a row.
func TestTIFFPredictorOneBitGray(t *testing.T) {
	const columns = 16 // no padding: compare against the encoder
	rowLen := 2
	raw := pattern(rowLen*3, 1)
	enc := tiffDiffRows(raw, rowLen, 1, 1, columns)
	if got := mustDecode(t, enc, decodeParms(2, 1, 1, columns)); !bytes.Equal(got, raw) {
		t.Fatalf("\n got % x\nwant % x", got, raw)
	}

	// Columns 3: samples 1, 1, 1 are stored as 1, 0, 0. PdfPig carries
	// the last value into the 5 padding bits.
	got := mustDecode(t, []byte{0b1000_0000, 0b0100_0000}, decodeParms(2, 1, 1, 3))
	want := []byte{0b1111_1111, 0b0111_1111}
	if !bytes.Equal(got, want) {
		t.Fatalf("padding bits: got %08b, want %08b", got, want)
	}
}

// TestTIFFPredictorTruncatedRow: a short final row keeps the previous
// row's bytes past its end, as in the PNG case.
func TestTIFFPredictorTruncatedRow(t *testing.T) {
	enc := []byte{10, 20, 30, 1, 2, 3, 5, 5}
	// Row 0 decodes to 10, 30, 60, 61, 63, 66. Row 1 is 5, 5, then
	// 60, 61, 63, 66 from row 0, then the running sum modulo 256.
	want := []byte{10, 30, 60, 61, 63, 66, 5, 10, 70, 131, 194, 4}
	got := mustDecode(t, enc, decodeParms(2, 1, 8, 6))
	if !bytes.Equal(got, want) {
		t.Fatalf("\n got %v\nwant %v", got, want)
	}
}

// TestPredictorDefaults: with only /Predictor, the defaults are Colors 1,
// BitsPerComponent 8 and Columns 1, so each row is one byte.
func TestPredictorDefaults(t *testing.T) {
	enc := []byte{0, 5, 2, 3, 2, 4}
	want := []byte{5, 8, 12}
	if got := mustDecode(t, enc, decodeParms(12, 0, 0, 0)); !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// Without /BitsPerComponent, a 2-column RGB row is 6 bytes (8 bits),
	// not 1 byte (1 bit).
	rgb := []byte{0, 1, 2, 3, 4, 5, 6, 2, 1, 1, 1, 1, 1, 1}
	wantRGB := []byte{1, 2, 3, 4, 5, 6, 2, 3, 4, 5, 6, 7}
	if got := mustDecode(t, rgb, decodeParms(12, 3, 0, 2)); !bytes.Equal(got, wantRGB) {
		t.Fatalf("default BitsPerComponent: got %v, want %v", got, wantRGB)
	}
	// A non-numeric value also gives the default.
	parms := decodeParms(12, 0, 0, 0)
	parms.Set("Colors", core.NewPdfName("RGB"))
	if got := mustDecode(t, enc, parms); !bytes.Equal(got, want) {
		t.Fatalf("non-numeric /Colors: got %v, want %v", got, want)
	}
}

// TestPredictorColorsCap: PdfPig caps /Colors at 32.
func TestPredictorColorsCap(t *testing.T) {
	raw := pattern(32*3, 32)
	enc := pngFilterRows(raw, 32, 32, []byte{1, 2, 4})
	got := mustDecode(t, enc, decodeParms(15, 1000, 8, 1))
	if !bytes.Equal(got, raw) {
		t.Fatalf("\n got % x\nwant % x", got, raw)
	}
}

// TestPredictorInvalidParams: parameters that cannot describe a row
// return the input unchanged, without a panic or a large allocation.
func TestPredictorInvalidParams(t *testing.T) {
	data := []byte{2, 1, 2, 3, 4, 5, 6, 2, 1, 1, 1, 1, 1, 1}
	with := func(d *core.PdfDictionary, key string, v core.PdfObject) *core.PdfDictionary {
		d.Set(key, v)
		return d
	}
	cases := []struct {
		name  string
		parms *core.PdfDictionary
	}{
		{"Predictor 3", decodeParms(3, 3, 8, 2)},
		{"Predictor 16", decodeParms(16, 3, 8, 2)},
		{"Predictor 0", decodeParms(0, 3, 8, 2)},
		{"Predictor -12", decodeParms(-12, 3, 8, 2)},
		{"Colors 0", with(decodeParms(15, 3, 8, 2), "Colors", core.NewPdfInteger(0))},
		{"Colors -1", decodeParms(15, -1, 8, 2)},
		{"Colors 2^40, capped to 32", decodeParms(15, 1<<40, 8, 2)},
		{"Columns 0", with(decodeParms(15, 3, 8, 2), "Columns", core.NewPdfInteger(0))},
		{"Columns -5", decodeParms(15, 3, 8, -5)},
		{"TIFF Columns -5", decodeParms(2, 3, 8, -5)},
		{"Columns 2^40", decodeParms(15, 3, 8, 1<<40)},
		// 2^62 columns of 8 bits is 2^65 bits, which wraps to 0 in int64.
		{"Columns 2^62", decodeParms(15, 1, 8, 1<<62)},
		{"TIFF Columns 2^62", decodeParms(2, 1, 8, 1<<62)},
		{"Columns 1e18", with(decodeParms(15, 3, 8, 2), "Columns", core.NewPdfReal(1e18))},
		{"Columns -1e18", with(decodeParms(2, 3, 8, 2), "Columns", core.NewPdfReal(-1e18))},
		{"BitsPerComponent 0", with(decodeParms(15, 3, 8, 2), "BitsPerComponent", core.NewPdfInteger(0))},
		{"BitsPerComponent 3", decodeParms(15, 3, 3, 2)},
		{"BitsPerComponent 12", decodeParms(2, 3, 12, 2)},
		{"BitsPerComponent -8", decodeParms(15, 3, -8, 2)},
		{"row longer than input", decodeParms(15, 3, 8, 5)},
		// 14 data bytes and the filter type byte do not fit in 14 bytes.
		{"PNG row as long as input", decodeParms(15, 1, 8, 14)},
		{"TIFF row longer than input", decodeParms(2, 3, 8, 5)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			got := mustDecode(t, data, tc.parms)
			runtime.ReadMemStats(&after)
			if !bytes.Equal(got, data) {
				t.Fatalf("got % x, want the input unchanged", got)
			}
			if n := after.TotalAlloc - before.TotalAlloc; n > 1<<20 {
				t.Fatalf("allocated %d bytes for a %d-byte input", n, len(data))
			}
		})
	}

	if got := mustDecode(t, nil, decodeParms(15, 3, 8, 2)); len(got) != 0 {
		t.Fatalf("empty input: got % x", got)
	}
}

// TestPredictorOutputBound: the output is never more than twice the
// input, for any row length that the input can hold.
func TestPredictorOutputBound(t *testing.T) {
	data := pattern(64, 9)
	for _, predictor := range []int{2, 15} {
		for columns := 1; columns <= 64; columns++ {
			got := mustDecode(t, data, decodeParms(predictor, 1, 8, columns))
			if len(got) > 2*len(data) {
				t.Fatalf("Predictor %d Columns %d: %d output bytes for %d input bytes",
					predictor, columns, len(got), len(data))
			}
		}
	}
}

// TestDecompressStreamAppliesRGBPredictor runs the full stream path:
// FlateDecode, then the predictor with the /Colors from /DecodeParms.
func TestDecompressStreamAppliesRGBPredictor(t *testing.T) {
	const columns, rows = 4, 3
	raw := pattern(columns*3*rows, 42)
	filtered := pngFilterRows(raw, columns*3, 3, []byte{1, 4, 3})
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(filtered)
	zw.Close()

	dict := core.NewPdfDictionary()
	dict.Set("Filter", core.NewPdfName("FlateDecode"))
	dict.Set("DecodeParms", decodeParms(15, 3, 8, columns))
	got, err := decompressStreamWithLimit(z.Bytes(), dict, -1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("\n got % x\nwant % x", got, raw)
	}
}
