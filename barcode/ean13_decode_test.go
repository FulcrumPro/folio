// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import "testing"

// decodeEAN13 reads a generated EAN-13 symbol back to its 13 digits. It walks
// the guard patterns, decodes the left half as L or G to recover both the digit
// and its parity, decodes the right half as R, and reconstructs the first digit
// from the left-half parity pattern.
func decodeEAN13(t *testing.T, bc *Barcode) string {
	t.Helper()
	row := bc.modules[0]
	i := leadingDark(row)

	if !bitsEqualAt(row, i, []bool{true, false, true}) {
		t.Fatalf("start guard not found at %d", i)
	}
	i += 3

	left := make([]int, 6)
	parity := make([]byte, 6)
	for k := range 6 {
		seg := row[i : i+7]
		d, par := -1, byte(0)
		for v := range 10 {
			if bitsEqual(seg, eanLEncoding[v]) {
				d, par = v, 'O'
				break
			}
			if bitsEqual(seg, eanGEncoding[v]) {
				d, par = v, 'E'
				break
			}
		}
		if d < 0 {
			t.Fatalf("left digit %d undecodable", k)
		}
		left[k], parity[k] = d, par
		i += 7
	}

	if !bitsEqualAt(row, i, []bool{false, true, false, true, false}) {
		t.Fatalf("center guard not found at %d", i)
	}
	i += 5

	right := make([]int, 6)
	for k := range 6 {
		seg := row[i : i+7]
		d := -1
		for v := range 10 {
			if bitsEqual(seg, eanREncoding[v]) {
				d = v
				break
			}
		}
		if d < 0 {
			t.Fatalf("right digit %d undecodable", k)
		}
		right[k] = d
		i += 7
	}

	if !bitsEqualAt(row, i, []bool{true, false, true}) {
		t.Fatalf("end guard not found at %d", i)
	}

	first := -1
	for d := range 10 {
		if ean13Parity[d] == string(parity) {
			first = d
			break
		}
	}
	if first < 0 {
		t.Fatalf("left-half parity %q matches no first digit", parity)
	}

	out := make([]byte, 0, 13)
	out = append(out, byte('0'+first))
	for _, d := range left {
		out = append(out, byte('0'+d))
	}
	for _, d := range right {
		out = append(out, byte('0'+d))
	}
	return string(out)
}

func TestEAN13RoundTrip(t *testing.T) {
	// One input per leading digit (0-9) so every parity pattern is exercised.
	for d := range 10 {
		in := string(byte('0'+d)) + "11111111111" // 12 digits
		bc, err := NewEAN13(in)
		if err != nil {
			t.Fatalf("NewEAN13(%q): %v", in, err)
		}
		want := in + string(byte('0'+ean13CheckDigit(in)))
		if got := decodeEAN13(t, bc); got != want {
			t.Errorf("round trip = %q, want %q", got, want)
		}
	}

	for _, in := range []string{"5901234123457", "4006381333931", "0123456789012"} {
		bc, err := NewEAN13(in)
		if err != nil {
			t.Fatalf("NewEAN13(%q): %v", in, err)
		}
		if got := decodeEAN13(t, bc); got != in {
			t.Errorf("round trip = %q, want %q", got, in)
		}
	}
}

// TestEAN13GoldenPattern pins the core module pattern (between quiet zones) for
// known inputs, so a change to the encoding or parity tables is caught.
func TestEAN13GoldenPattern(t *testing.T) {
	cases := map[string]string{
		"5901234123457": "10100010110100111011001100100110111101001110101010110011011011001000010101110010011101000100101",
		"0123456789012": "10100110010010011011110101000110110001010111101010100010010010001110100111001011001101101100101",
	}
	for in, want := range cases {
		bc, err := NewEAN13(in)
		if err != nil {
			t.Fatalf("NewEAN13(%q): %v", in, err)
		}
		if got := coreBits(bc); got != want {
			t.Errorf("%s: core modules =\n%s\nwant\n%s", in, got, want)
		}
	}
}

func TestEAN13QuietZones(t *testing.T) {
	bc, err := NewEAN13("5901234123457")
	if err != nil {
		t.Fatal(err)
	}
	row := bc.modules[0]
	if lead, trail := leadingDark(row), trailingLight(row); lead != 11 || trail != 7 {
		t.Errorf("quiet zones = %d left / %d right, want 11/7", lead, trail)
	}
}
