// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import "testing"

// GF(256) arithmetic for QR error correction, primitive polynomial 0x11D.

var tExp [512]byte
var tLog [256]byte

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		tExp[i] = byte(x)
		tLog[x] = byte(i)
		x <<= 1
		if x >= 256 {
			x ^= 0x11D
		}
	}
	for i := 255; i < 512; i++ {
		tExp[i] = tExp[i-255]
	}
}

func tMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return tExp[int(tLog[a])+int(tLog[b])]
}

// rsValid reports whether codeword (data codewords followed by ecLen error
// correction codewords, highest-degree coefficient first) is a valid
// Reed-Solomon codeword, i.e. every syndrome C(alpha^j) for j in [0,ecLen) is zero.
func rsValid(codeword []byte, ecLen int) bool {
	for j := 0; j < ecLen; j++ {
		var s byte
		root := tExp[j] // alpha^j
		for _, c := range codeword {
			s = tMul(s, root) ^ c // Horner evaluation
		}
		if s != 0 {
			return false
		}
	}
	return true
}

// qrTotalCodewords is the total number of (data + error correction) codewords
// for each QR version 1-40.
var qrTotalCodewords = [41]int{
	0,
	26, 44, 70, 100, 134, 172, 196, 242, 292, 346,
	404, 466, 532, 581, 655, 733, 815, 901, 991, 1085,
	1156, 1258, 1364, 1474, 1588, 1706, 1828, 1921, 2051, 2185,
	2323, 2465, 2611, 2761, 2876, 3034, 3196, 3362, 3532, 3706,
}

// TestQRReedSolomonVector checks rsEncode for a version 1, level Q data block
// against its expected error correction codewords.
func TestQRReedSolomonVector(t *testing.T) {
	data := []byte{0x20, 0x5B, 0x0B, 0x78, 0xD1, 0x72, 0xDC, 0x4D, 0x43, 0x40, 0xEC, 0x11, 0xEC}
	want := []byte{0xA8, 0x48, 0x16, 0x52, 0xD9, 0x36, 0x9C, 0x00, 0x2E, 0x0F, 0xB4, 0x7A, 0x10}
	got := rsEncode(data, rsGeneratorPoly(13), 13)
	if len(got) != len(want) {
		t.Fatalf("ecc length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ecc = % X, want % X", got, want)
		}
	}
}

// TestQRCodewordTables checks, for every version and ECC level, that the
// per-block data codewords sum to qrDataCodewords and that data plus error
// correction codewords equal the total codeword count.
func TestQRCodewordTables(t *testing.T) {
	for v := 1; v <= 40; v++ {
		for l := ECCLevelL; l <= ECCLevelH; l++ {
			bi := qrBlockTable[v][l]
			blocks := bi.group1Blocks + bi.group2Blocks
			dataCW := bi.group1Blocks*bi.group1DataCW + bi.group2Blocks*bi.group2DataCW
			if dataCW != qrDataCodewords[v][l] {
				t.Errorf("v%d L%d: block data codewords = %d, qrDataCodewords = %d", v, l, dataCW, qrDataCodewords[v][l])
			}
			total := dataCW + blocks*qrECCPerBlock[v][l]
			if total != qrTotalCodewords[v] {
				t.Errorf("v%d L%d: data+ecc codewords = %d, want total %d", v, l, total, qrTotalCodewords[v])
			}
		}
	}
}

// qrFunctionMap returns the function-pattern reservation for a version.
func qrFunctionMap(version, size int) [][]bool {
	modules := make([][]bool, size)
	reserved := make([][]bool, size)
	for i := range reserved {
		modules[i] = make([]bool, size)
		reserved[i] = make([]bool, size)
	}
	placeFinder(modules, reserved, 0, 0)
	placeFinder(modules, reserved, 0, size-7)
	placeFinder(modules, reserved, size-7, 0)
	if version >= 2 {
		pos := alignmentPositions(version)
		for _, r := range pos {
			for _, c := range pos {
				if !reserved[r][c] {
					placeAlignment(modules, reserved, r, c)
				}
			}
		}
	}
	for i := 8; i < size-8; i++ {
		reserved[6][i] = true
		reserved[i][6] = true
	}
	reserved[size-8][8] = true
	for i := 0; i < 9; i++ {
		reserved[8][i] = true
		reserved[i][8] = true
	}
	for i := 0; i < 8; i++ {
		reserved[8][size-1-i] = true
		reserved[size-1-i][8] = true
	}
	if version >= 7 {
		for i := 0; i < 6; i++ {
			for j := 0; j < 3; j++ {
				reserved[size-11+j][i] = true
				reserved[i][size-11+j] = true
			}
		}
	}
	return reserved
}

// readFormat reads the 15-bit format information from both standard copies and
// returns (level, mask, ok). ok is false if the two copies disagree or the
// value is not a valid format string.
func readFormat(m [][]bool, size int) (ECCLevel, int, bool) {
	pos1 := [15][2]int{
		{0, 8}, {1, 8}, {2, 8}, {3, 8}, {4, 8}, {5, 8}, {7, 8}, {8, 8},
		{8, 7}, {8, 5}, {8, 4}, {8, 3}, {8, 2}, {8, 1}, {8, 0},
	}
	var v1 uint16
	for i, p := range pos1 {
		if m[p[0]][p[1]] {
			v1 |= 1 << i
		}
	}
	var v2 uint16
	for i := 0; i < 8; i++ {
		if m[8][size-1-i] {
			v2 |= 1 << i
		}
	}
	for i := 8; i < 15; i++ {
		if m[size-15+i][8] {
			v2 |= 1 << i
		}
	}
	if v1 != v2 {
		return 0, 0, false
	}
	for level := ECCLevelL; level <= ECCLevelH; level++ {
		for mask := 0; mask < 8; mask++ {
			if qrFormatInfo[level][mask] == v1 {
				return level, mask, true
			}
		}
	}
	return 0, 0, false
}

// readVersionInfo reads the 18-bit version information from the top-right block.
func readVersionInfo(m [][]bool, size int) uint32 {
	var info uint32
	for i := 0; i < 18; i++ {
		r := i / 3
		c := i % 3
		if m[r][size-11+c] {
			info |= 1 << i
		}
	}
	return info
}

// extractCodewordStream reads the interleaved codeword bytes from a symbol,
// undoing the data mask and skipping function modules.
func extractCodewordStream(m, reserved [][]bool, size, mask int) []byte {
	var bits []bool
	upward := true
	for col := size - 1; col >= 0; col -= 2 {
		if col == 6 {
			col--
		}
		if col < 0 {
			break
		}
		for k := 0; k < size; k++ {
			row := k
			if upward {
				row = size - 1 - k
			}
			for c := col; c >= max(col-1, 0); c-- {
				if reserved[row][c] {
					continue
				}
				v := m[row][c]
				if qrMaskFunc(mask, row, c) {
					v = !v
				}
				bits = append(bits, v)
			}
		}
		upward = !upward
	}
	out := make([]byte, len(bits)/8)
	for i := range out {
		var b byte
		for j := 0; j < 8; j++ {
			if bits[i*8+j] {
				b |= 1 << (7 - j)
			}
		}
		out[i] = b
	}
	return out
}

// deinterleaveBlocks reverses the codeword interleaving, returning each block as
// its data codewords followed by its error correction codewords.
func deinterleaveBlocks(stream []byte, version int, level ECCLevel) [][]byte {
	bi := qrBlockTable[version][level]
	ecPerBlock := qrECCPerBlock[version][level]
	totalBlocks := bi.group1Blocks + bi.group2Blocks

	dataLens := make([]int, totalBlocks)
	for i := 0; i < bi.group1Blocks; i++ {
		dataLens[i] = bi.group1DataCW
	}
	for i := 0; i < bi.group2Blocks; i++ {
		dataLens[bi.group1Blocks+i] = bi.group2DataCW
	}

	blocks := make([][]byte, totalBlocks)
	for i := range blocks {
		blocks[i] = make([]byte, 0, dataLens[i]+ecPerBlock)
	}

	maxData := bi.group1DataCW
	if bi.group2DataCW > maxData {
		maxData = bi.group2DataCW
	}
	idx := 0
	for j := 0; j < maxData; j++ {
		for i := 0; i < totalBlocks; i++ {
			if j < dataLens[i] {
				blocks[i] = append(blocks[i], stream[idx])
				idx++
			}
		}
	}
	for j := 0; j < ecPerBlock; j++ {
		for i := 0; i < totalBlocks; i++ {
			blocks[i] = append(blocks[i], stream[idx])
			idx++
		}
	}
	return blocks
}

// TestQRDecodeRoundTrip generates QR codes spanning all modes, ECC levels, and a
// range of versions, then verifies each one decodes structurally: the format
// reads back identically from both copies, version information (v7+) matches,
// and every Reed-Solomon block is a valid codeword.
func TestQRDecodeRoundTrip(t *testing.T) {
	repeat := func(n int, cs string) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = cs[i%len(cs)]
		}
		return string(b)
	}
	type tc struct {
		name string
		data string
	}
	cases := []tc{
		{"url", "https://example.com/folio"},
		{"numeric", "1234567890"},
		{"alphanumeric", "HELLO WORLD $%*+-./:"},
		{"byte", "Hello, World! lowercase forces byte mode."},
		{"v7-byte", repeat(180, "abcdefghijklmnopqrstuvwxyz")},   // forces version info
		{"v10-numeric", repeat(700, "0123456789")},               // multi-block
		{"v21H-byte", repeat(400, "abcdefghijklmnopqrstuvwxyz")}, // high version, level H
		{"v32-byte", repeat(1100, "abcdefghijklmnopqrstuvwxyz")}, // very high version
	}
	levels := []ECCLevel{ECCLevelL, ECCLevelM, ECCLevelQ, ECCLevelH}
	for _, c := range cases {
		for _, level := range levels {
			bc, err := NewQRWithECC(c.data, level)
			if err != nil {
				t.Fatalf("%s L%d: %v", c.name, level, err)
			}
			size := bc.width
			version := (size - 17) / 4
			m := bc.modules

			gotLevel, mask, ok := readFormat(m, size)
			if !ok {
				t.Errorf("%s L%d: format info invalid or copies disagree", c.name, level)
				continue
			}
			if gotLevel != level {
				t.Errorf("%s L%d: format reads level %d", c.name, level, gotLevel)
			}
			if version >= 7 {
				if got := readVersionInfo(m, size); got != qrVersionInfo[version] {
					t.Errorf("%s L%d: version info = 0x%05X, want 0x%05X", c.name, level, got, qrVersionInfo[version])
				}
			}

			reserved := qrFunctionMap(version, size)
			stream := extractCodewordStream(m, reserved, size, mask)
			if len(stream) < qrTotalCodewords[version] {
				t.Errorf("%s L%d: extracted %d codewords, want >= %d", c.name, level, len(stream), qrTotalCodewords[version])
				continue
			}
			stream = stream[:qrTotalCodewords[version]]
			for bn, blk := range deinterleaveBlocks(stream, version, level) {
				if !rsValid(blk, qrECCPerBlock[version][level]) {
					t.Errorf("%s L%d: block %d is not a valid Reed-Solomon codeword", c.name, level, bn)
				}
			}
		}
	}
}
