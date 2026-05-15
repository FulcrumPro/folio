// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "testing"

func FuzzParseTTF(f *testing.F) {
	// Seed with empty bytes.
	f.Add([]byte{})
	// Seed with the TrueType magic number (scalar type 1).
	f.Add([]byte{0x00, 0x01, 0x00, 0x00})
	// Seed with the "true" tag used by some TrueType fonts.
	f.Add([]byte("true"))
	// Seed with the OpenType magic number.
	f.Add([]byte("OTTO"))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseTTF panicked: %v", r)
			}
		}()
		// Errors are expected for random input; only panics are failures.
		_, _ = ParseTTF(data)
	})
}

// FuzzParseCFF exercises the CID-keyed CFF parser. Random input
// should never cause a panic — parseCFF must walk every operand and
// offset defensively, since Phase 3+ will trust its output to
// produce a valid PDF font stream.
func FuzzParseCFF(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x01, 0x00, 0x04, 0x02})
	f.Add(buildSyntheticCFFv1([]byte{139, 139, 139, 12, 30}))
	// Seed a complete valid CID-keyed blob so the fuzzer has a
	// known-good starting point to mutate.
	f.Add(buildSyntheticCIDKeyedCFF(testingTNoop{}, syntheticCFFOptions{numGlyphs: 3, fdCount: 1}))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("parseCFF panicked on %d bytes: %v", len(data), r)
			}
		}()
		_, _ = parseCFF(data)
	})
}

// FuzzParseCFFDict targets the DICT operand/operator stream parser
// directly. Integer overflow, BCD-end-nibble corner cases, and
// reserved-byte handling are all in scope.
func FuzzParseCFFDict(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{139, 0})                          // int 0, version
	f.Add([]byte{28, 0x05, 0xDC, 0})               // shortint 1500
	f.Add([]byte{29, 0, 0, 0, 0, 0})               // longint 0
	f.Add([]byte{30, 0x1A, 0xFF, 0})               // BCD real "1." end
	f.Add([]byte{139, 139, 139, 12, 30})           // ROS
	f.Add([]byte{251, 0, 0})                       // negative 2-byte int
	f.Add([]byte{247, 255, 0})                     // positive 2-byte int

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("parseCFFDict panicked on %d bytes: %v", len(data), r)
			}
		}()
		_, _ = parseCFFDict(data)
	})
}

// testingTNoop implements fixtureT for the synthetic CFF builder when
// called from a fuzz seed list. The builder only calls Helper /
// Fatalf on bad inputs that we never pass in seed corpus, so a no-op
// implementation is safe here.
type testingTNoop struct{}

func (testingTNoop) Helper()              {}
func (testingTNoop) Fatalf(string, ...any) {}
