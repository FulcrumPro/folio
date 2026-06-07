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
// table, checks the start code, modulo-103 checksum, and stop pattern, then
// interprets the values according to the active code set, following A/B/C
// switches.
func decodeCode128(t *testing.T, bc *Barcode) string {
	t.Helper()
	row := bc.modules[0]
	lo := leadingDark(row)
	hi := len(row) - trailingLight(row)
	sym := row[lo:hi]

	if len(sym) < 11*2+13 {
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

	data := values[1 : len(values)-1]
	want := values[len(values)-1]
	sum := values[0]
	for k, v := range data {
		sum += v * (k + 1)
	}
	if sum%103 != want {
		t.Fatalf("checksum = %d, encoded %d", sum%103, want)
	}

	var set c128Set
	switch values[0] {
	case c128StartA:
		set = c128A
	case c128StartB:
		set = c128B
	case c128StartC:
		set = c128C
	default:
		t.Fatalf("unexpected start code %d", values[0])
	}

	var out []byte
	for _, v := range data {
		switch set {
		case c128C:
			switch {
			case v <= 99:
				out = append(out, byte('0'+v/10), byte('0'+v%10))
			case v == c128SwitchB:
				set = c128B
			case v == c128SwitchA:
				set = c128A
			default:
				t.Fatalf("unexpected Code C symbol %d", v)
			}
		case c128B:
			switch {
			case v <= 95:
				out = append(out, byte(v+32))
			case v == c128SwitchC:
				set = c128C
			case v == c128SwitchA:
				set = c128A
			default:
				t.Fatalf("unexpected Code B symbol %d", v)
			}
		default: // c128A
			switch {
			case v <= 63:
				out = append(out, byte(v+32))
			case v <= 95:
				out = append(out, byte(v-64)) // control characters
			case v == c128SwitchC:
				set = c128C
			case v == c128SwitchB:
				set = c128B
			default:
				t.Fatalf("unexpected Code A symbol %d", v)
			}
		}
	}
	return string(out)
}

func TestCode128RoundTrip(t *testing.T) {
	inputs := []string{
		"FOLIO-2026",         // Code B then Code C
		"Hello, World!",      // Code B
		"1234567890",         // all digits, Code C
		"12345",              // odd digit run
		"AB12CD",             // short digit run stays in Code B
		"ABCD1234567890WXYZ", // Code B, Code C, Code B
		"ABC abc 123",        // mixed case and a short digit run
		" !\"#$%&'()*+,-./",  // Code B symbols
		":;<=>?@[\\]^_`{|}~", // Code B symbols
		"\x7f",               // DEL, the top of Code B
		"\x00\x01\x1f",       // Code A control characters
		"line1\nline2\ttab",  // control characters mixed with Code B text
		"\t\t007 agent",      // Code A then Code B
		"\x011234",           // Code A then Code C
		"1234\x01",           // Code C then Code A
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
// for known inputs across code sets B, C, and A, so a change to the pattern
// tables is caught.
func TestCode128GoldenPattern(t *testing.T) {
	cases := map[string]string{
		"FOLIO-2026": "11010010000100011000101000111011010001101110110001000101000111011010011011100101110111101100100111011100100110110000101001100011101011",
		"1234567890": "110100111001011001110010001011000111000101101100001010011011110110100111100101100011101011",
		"AB\x01CD":   "11010010000101000110001000101100011101011110100101100001000100011010110001000111001001101100011101011",
	}
	for in, want := range cases {
		bc, err := NewCode128(in)
		if err != nil {
			t.Fatalf("NewCode128(%q): %v", in, err)
		}
		if got := coreBits(bc); got != want {
			t.Errorf("%q: core modules =\n%s\nwant\n%s", in, got, want)
		}
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
