// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import "testing"

// bitsEqual reports whether two module slices are identical.
func bitsEqual(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// bitsEqualAt reports whether pat matches row starting at offset off.
func bitsEqualAt(row []bool, off int, pat []bool) bool {
	if off+len(pat) > len(row) {
		return false
	}
	return bitsEqual(row[off:off+len(pat)], pat)
}

// leadingDark returns the index of the first dark module, i.e. the width of the
// leading light (quiet) zone.
func leadingDark(row []bool) int {
	i := 0
	for i < len(row) && !row[i] {
		i++
	}
	return i
}

// trailingLight returns the width of the trailing light (quiet) zone.
func trailingLight(row []bool) int {
	n := 0
	for i := len(row) - 1; i >= 0 && !row[i]; i-- {
		n++
	}
	return n
}

// coreBits returns the module row between the quiet zones as a string of '1'
// (dark) and '0' (light).
func coreBits(bc *Barcode) string {
	row := bc.modules[0]
	lo := leadingDark(row)
	hi := len(row) - trailingLight(row)
	b := make([]byte, 0, hi-lo)
	for _, v := range row[lo:hi] {
		if v {
			b = append(b, '1')
		} else {
			b = append(b, '0')
		}
	}
	return string(b)
}

// decodeCode128 reads a generated Code 128 symbol back to its text. It segments
// the symbol into 11-module characters, recovers each value from the pattern
// table, checks the start code, modulo-103 checksum, and stop pattern, and maps
// the data values back to ASCII.
func decodeCode128(t *testing.T, bc *Barcode) string {
	t.Helper()
	row := bc.modules[0]
	lo := leadingDark(row)
	hi := len(row) - trailingLight(row)
	sym := row[lo:hi]

	if len(sym) < 11*3+13 {
		t.Fatalf("symbol too short: %d modules", len(sym))
	}
	if !bitsEqual(sym[len(sym)-13:], code128Stop) {
		t.Fatalf("stop pattern mismatch")
	}
	body := sym[:len(sym)-13]
	if len(body)%11 != 0 {
		t.Fatalf("character region %d not a multiple of 11", len(body))
	}

	values := make([]int, 0, len(body)/11)
	for i := 0; i < len(body); i += 11 {
		v := -1
		for cand, pat := range code128Patterns {
			if bitsEqual(body[i:i+11], pat) {
				v = cand
				break
			}
		}
		if v < 0 {
			t.Fatalf("unknown character pattern at module %d", i)
		}
		values = append(values, v)
	}

	if values[0] != 104 {
		t.Fatalf("expected Start B (104), got %d", values[0])
	}
	data := values[1 : len(values)-1]
	want := values[len(values)-1]
	sum := 104
	for k, v := range data {
		sum += v * (k + 1)
	}
	if sum%103 != want {
		t.Fatalf("checksum = %d, encoded %d", sum%103, want)
	}

	out := make([]byte, len(data))
	for i, v := range data {
		out[i] = byte(v + 32)
	}
	return string(out)
}

func TestCode128RoundTrip(t *testing.T) {
	inputs := []string{
		"FOLIO-2026",
		"Hello, World!",
		"1234567890",
		"ABC abc 123",
		" !\"#$%&'()*+,-./",
		":;<=>?@[\\]^_`{|}~",
		"\x7f", // DEL, the top of Code B
	}
	for _, in := range inputs {
		bc, err := NewCode128(in)
		if err != nil {
			t.Fatalf("NewCode128(%q): %v", in, err)
		}
		if got := decodeCode128(t, bc); got != in {
			t.Errorf("round trip = %q, want %q", got, in)
		}
	}
}

func TestCode128RoundTripAllPrintable(t *testing.T) {
	data := make([]byte, 0, 95)
	for ch := byte(32); ch < 127; ch++ {
		data = append(data, ch)
	}
	bc, err := NewCode128(string(data))
	if err != nil {
		t.Fatalf("NewCode128: %v", err)
	}
	if got := decodeCode128(t, bc); got != string(data) {
		t.Errorf("round trip = %q, want %q", got, data)
	}
}

// TestCode128GoldenPattern pins the core module pattern (between quiet zones)
// for a known input, so a change to the pattern tables is caught.
func TestCode128GoldenPattern(t *testing.T) {
	const want = "1101001000010001100010100011101101000110111011000100010100011101101001101110011001110010100111011001100111001011001110100110100010001100011101011"
	bc, err := NewCode128("FOLIO-2026")
	if err != nil {
		t.Fatal(err)
	}
	if got := coreBits(bc); got != want {
		t.Errorf("core modules =\n%s\nwant\n%s", got, want)
	}
}

func TestCode128QuietZones(t *testing.T) {
	bc, err := NewCode128("X")
	if err != nil {
		t.Fatal(err)
	}
	row := bc.modules[0]
	if lead, trail := leadingDark(row), trailingLight(row); lead != 10 || trail != 10 {
		t.Errorf("quiet zones = %d left / %d right, want 10/10", lead, trail)
	}
}
