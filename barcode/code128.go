// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import "fmt"

// Code 128 symbol values for the start codes and code-set switches. A switch
// value selects the named code set from whichever set is currently active.
const (
	c128StartA  = 103
	c128StartB  = 104
	c128StartC  = 105
	c128SwitchC = 99
	c128SwitchB = 100
	c128SwitchA = 101
)

// c128Set identifies a Code 128 code set.
type c128Set int

const (
	c128None c128Set = iota
	c128A
	c128B
	c128C
)

// NewCode128 generates a Code 128 barcode from a string. It encodes the full
// ASCII range (0-127) by selecting code sets A, B, and C automatically, using
// Code C for runs of digits. Bytes above 127 return an error.
func NewCode128(data string) (*Barcode, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("barcode: empty data")
	}

	symbols, err := encodeCode128(data)
	if err != nil {
		return nil, err
	}

	var modules []bool

	// Left quiet zone (10 modules).
	for range 10 {
		modules = append(modules, false)
	}

	// Symbol values, accumulating the modulo-103 checksum (start code has
	// weight 1, each following symbol weight 1, 2, 3, ...).
	checksum := symbols[0]
	modules = append(modules, code128Patterns[symbols[0]]...)
	for k, v := range symbols[1:] {
		modules = append(modules, code128Patterns[v]...)
		checksum += v * (k + 1)
	}
	modules = append(modules, code128Patterns[checksum%103]...)

	// Stop pattern (13 modules: 2331112).
	modules = append(modules, code128Stop...)

	// Right quiet zone (10 modules).
	for range 10 {
		modules = append(modules, false)
	}

	return new1D(modules, 50), nil
}

// isDigit128 reports whether b is an ASCII digit.
func isDigit128(b byte) bool { return b >= '0' && b <= '9' }

// digitRun128 returns the number of consecutive ASCII digits in data from i.
func digitRun128(data string, i int) int {
	n := 0
	for i+n < len(data) && isDigit128(data[i+n]) {
		n++
	}
	return n
}

// valueA returns the Code A value for an ASCII byte 0-95: 32-95 map to 0-63,
// and the control characters 0-31 map to 64-95.
func valueA(b byte) int {
	if b < 32 {
		return int(b) + 64
	}
	return int(b) - 32
}

// encodeCode128 converts data to Code 128 symbol values, beginning with a start
// code and ending before the checksum. Code C is used for digit runs, Code A
// for control characters, and Code B otherwise.
func encodeCode128(data string) ([]int, error) {
	for i := range len(data) {
		if data[i] > 127 {
			return nil, fmt.Errorf("barcode: Code 128 does not support byte %d at position %d", data[i], i)
		}
	}

	n := len(data)
	var symbols []int
	set := c128None

	for i := 0; i < n; {
		if set == c128C {
			// In Code C a value 0-99 is a digit pair. The encoder never
			// switches C to C, so the value 99 is always the pair "99" and not
			// the switch-to-C code.
			if i+1 < n && isDigit128(data[i]) && isDigit128(data[i+1]) {
				symbols = append(symbols, int(data[i]-'0')*10+int(data[i+1]-'0'))
				i += 2
				continue
			}
			// Leave Code C for the character at i.
			if data[i] < 32 {
				symbols = append(symbols, c128SwitchA)
				set = c128A
			} else {
				symbols = append(symbols, c128SwitchB)
				set = c128B
			}
			continue
		}

		run := digitRun128(data, i)
		// Code C pays off for runs of four or more digits, or for an all-digit
		// payload of even length.
		wantC := run >= 4 || (run == n && run >= 2 && run%2 == 0)

		if set == c128None {
			switch {
			case wantC:
				symbols = append(symbols, c128StartC)
				set = c128C
			case data[i] < 32:
				symbols = append(symbols, c128StartA)
				set = c128A
			default:
				symbols = append(symbols, c128StartB)
				set = c128B
			}
			continue
		}

		if run >= 4 {
			symbols = append(symbols, c128SwitchC)
			set = c128C
			continue
		}

		c := data[i]
		switch set {
		case c128A:
			if c <= 95 {
				symbols = append(symbols, valueA(c))
				i++
			} else { // 96-127 needs Code B
				symbols = append(symbols, c128SwitchB)
				set = c128B
			}
		default: // c128B
			if c >= 32 {
				symbols = append(symbols, int(c)-32)
				i++
			} else { // control character needs Code A
				symbols = append(symbols, c128SwitchA)
				set = c128A
			}
		}
	}

	return symbols, nil
}

// code128Patterns contains the bar/space patterns for Code 128.
// Each pattern is 11 modules (6 alternating bars and spaces).
// Index 0-105 = data/control characters.
// Patterns sourced from ISO/IEC 15417 (Code 128 specification).
var code128Patterns = [106][]bool{
	{true, true, false, true, true, false, false, true, true, false, false},   // 0
	{true, true, false, false, true, true, false, true, true, false, false},   // 1
	{true, true, false, false, true, true, false, false, true, true, false},   // 2
	{true, false, false, true, false, false, true, true, false, false, false}, // 3
	{true, false, false, true, false, false, false, true, true, false, false}, // 4
	{true, false, false, false, true, false, false, true, true, false, false}, // 5
	{true, false, false, true, true, false, false, true, false, false, false}, // 6
	{true, false, false, true, true, false, false, false, true, false, false}, // 7
	{true, false, false, false, true, true, false, false, true, false, false}, // 8
	{true, true, false, false, true, false, false, true, false, false, false}, // 9
	{true, true, false, false, true, false, false, false, true, false, false}, // 10
	{true, true, false, false, false, true, false, false, true, false, false}, // 11
	{true, false, true, true, false, false, true, true, true, false, false},   // 12
	{true, false, false, true, true, false, true, true, true, false, false},   // 13
	{true, false, false, true, true, false, false, true, true, true, false},   // 14
	{true, false, true, true, true, false, false, true, true, false, false},   // 15
	{true, false, false, true, true, true, false, true, true, false, false},   // 16
	{true, false, false, true, true, true, false, false, true, true, false},   // 17
	{true, true, false, false, true, true, true, false, false, true, false},   // 18
	{true, true, false, false, true, false, true, true, true, false, false},   // 19
	{true, true, false, false, true, false, false, true, true, true, false},   // 20
	{true, true, false, true, true, true, false, false, true, false, false},   // 21
	{true, true, false, false, true, true, true, false, true, false, false},   // 22
	{true, true, true, false, true, true, false, true, true, true, false},     // 23
	{true, true, true, false, true, false, false, true, true, false, false},   // 24
	{true, true, true, false, false, true, false, true, true, false, false},   // 25
	{true, true, true, false, false, true, false, false, true, true, false},   // 26
	{true, true, true, false, true, true, false, false, true, false, false},   // 27
	{true, true, true, false, false, true, true, false, true, false, false},   // 28
	{true, true, true, false, false, true, true, false, false, true, false},   // 29
	{true, true, false, true, true, false, true, true, false, false, false},   // 30
	{true, true, false, true, true, false, false, false, true, true, false},   // 31
	{true, true, false, false, false, true, true, false, true, true, false},   // 32
	{true, false, true, false, false, false, true, true, false, false, false}, // 33
	{true, false, false, false, true, false, true, true, false, false, false}, // 34
	{true, false, false, false, true, false, false, false, true, true, false}, // 35
	{true, false, true, true, false, false, false, true, false, false, false}, // 36
	{true, false, false, false, true, true, false, true, false, false, false}, // 37
	{true, false, false, false, true, true, false, false, false, true, false}, // 38
	{true, true, false, true, false, false, false, true, false, false, false}, // 39
	{true, true, false, false, false, true, false, true, false, false, false}, // 40
	{true, true, false, false, false, true, false, false, false, true, false}, // 41
	{true, false, true, true, false, true, true, true, false, false, false},   // 42
	{true, false, true, true, false, false, false, true, true, true, false},   // 43
	{true, false, false, false, true, true, false, true, true, true, false},   // 44
	{true, false, true, true, true, false, true, true, false, false, false},   // 45
	{true, false, true, true, true, false, false, false, true, true, false},   // 46
	{true, false, false, false, true, true, true, false, true, true, false},   // 47
	{true, true, true, false, true, true, true, false, true, true, false},     // 48
	{true, true, false, true, false, false, false, true, true, true, false},   // 49
	{true, true, false, false, false, true, false, true, true, true, false},   // 50
	{true, true, false, true, true, true, false, true, false, false, false},   // 51
	{true, true, false, true, true, true, false, false, false, true, false},   // 52
	{true, true, false, true, true, true, false, true, true, true, false},     // 53
	{true, true, true, false, true, false, true, true, false, false, false},   // 54
	{true, true, true, false, true, false, false, false, true, true, false},   // 55
	{true, true, true, false, false, false, true, false, true, true, false},   // 56
	{true, true, true, false, true, true, false, true, false, false, false},   // 57
	{true, true, true, false, true, true, false, false, false, true, false},   // 58
	{true, true, true, false, false, false, true, true, false, true, false},   // 59
	{true, true, true, false, true, true, true, true, false, true, false},     // 60
	{true, true, false, false, true, false, false, false, false, true, false}, // 61
	{true, true, true, true, false, false, false, true, false, true, false},   // 62
	{true, false, true, false, false, true, true, false, false, false, false}, // 63
	{true, false, true, false, false, false, false, true, true, false, false}, // 64
	{true, false, false, true, false, true, true, false, false, false, false}, // 65
	{true, false, false, true, false, false, false, false, true, true, false}, // 66
	{true, false, false, false, false, true, false, true, true, false, false}, // 67
	{true, false, false, false, false, true, false, false, true, true, false}, // 68
	{true, false, true, true, false, false, true, false, false, false, false}, // 69
	{true, false, true, true, false, false, false, false, true, false, false}, // 70
	{true, false, false, true, true, false, true, false, false, false, false}, // 71
	{true, false, false, true, true, false, false, false, false, true, false}, // 72
	{true, false, false, false, false, true, true, false, true, false, false}, // 73
	{true, false, false, false, false, true, true, false, false, true, false}, // 74
	{true, true, false, false, false, false, true, false, false, true, false}, // 75
	{true, true, false, false, true, false, true, false, false, false, false}, // 76
	{true, true, true, true, false, true, true, true, false, true, false},     // 77
	{true, true, false, false, false, false, true, false, true, false, false}, // 78
	{true, false, false, false, true, true, true, true, false, true, false},   // 79
	{true, false, true, false, false, true, true, true, true, false, false},   // 80
	{true, false, false, true, false, true, true, true, true, false, false},   // 81
	{true, false, false, true, false, false, true, true, true, true, false},   // 82
	{true, false, true, true, true, true, false, false, true, false, false},   // 83
	{true, false, false, true, true, true, true, false, true, false, false},   // 84
	{true, false, false, true, true, true, true, false, false, true, false},   // 85
	{true, true, true, true, false, true, false, false, true, false, false},   // 86
	{true, true, true, true, false, false, true, false, true, false, false},   // 87
	{true, true, true, true, false, false, true, false, false, true, false},   // 88
	{true, true, false, true, true, false, true, true, true, true, false},     // 89
	{true, true, false, true, true, true, true, false, true, true, false},     // 90
	{true, true, true, true, false, true, true, false, true, true, false},     // 91
	{true, false, true, false, true, true, true, true, false, false, false},   // 92
	{true, false, true, false, false, false, true, true, true, true, false},   // 93
	{true, false, false, false, true, false, true, true, true, true, false},   // 94
	{true, false, true, true, true, true, false, true, false, false, false},   // 95
	{true, false, true, true, true, true, false, false, false, true, false},   // 96
	{true, true, true, true, false, true, false, true, false, false, false},   // 97
	{true, true, true, true, false, true, false, false, false, true, false},   // 98
	{true, false, true, true, true, false, true, true, true, true, false},     // 99
	{true, false, true, true, true, true, false, true, true, true, false},     // 100
	{true, true, true, false, true, false, true, true, true, true, false},     // 101
	{true, true, true, true, false, true, false, true, true, true, false},     // 102
	{true, true, false, true, false, false, false, false, true, false, false}, // 103: Start A
	{true, true, false, true, false, false, true, false, false, false, false}, // 104: Start B
	{true, true, false, true, false, false, true, true, true, false, false},   // 105: Start C
}

// code128Stop is the stop pattern (13 modules: 2331112).
var code128Stop = []bool{
	true, true, false, false, false, true, true, true, false, true, false, true, true,
}
