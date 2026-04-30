// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"errors"
	"testing"
)

// TestParseFontRejectsType1Magic pins the fix for issue #230. PostScript
// Type 1 fonts (magic 0x74797031, "typ1") were previously listed in
// ParseFont's dispatch alongside TrueType variants, but ParseTTF routes
// them to sfnt.Parse which rejects Type 1 with a confusing
// "parse font: ..." error. The dispatch was a false advertisement — the
// format was claimed but never delivered. With the entry removed,
// callers feeding Type 1 bytes get a clear ErrUnknownFormat instead.
//
// Type 1 has been deprecated since PDF 2.0; no modern font foundry
// ships in this format. Re-introducing typ1 support would require a
// full Type 1 parser, not just adding the magic back.
func TestParseFontRejectsType1Magic(t *testing.T) {
	data := make([]byte, 16)
	binary.BigEndian.PutUint32(data[0:4], 0x74797031)
	_, err := ParseFont(data)
	if err == nil {
		t.Fatal("expected error for typ1 magic")
	}
	if !errors.Is(err, ErrUnknownFormat) {
		t.Errorf("expected errors.Is(err, ErrUnknownFormat), got %v", err)
	}
}

// TestParseFontDispatchSurface audits every magic the dispatch claims
// to support. For each, the test feeds 16 bytes whose first 4 bytes
// match the magic but whose body is otherwise empty/invalid; the
// expectation is "the dispatcher routed to a parser" — concretely,
// the resulting error must NOT be ErrUnknownFormat. A subsequent parse
// failure (truncated table directory, bad sfnt header, etc.) is fine
// and expected; the test pins only that the magic is honored.
//
// This catches the failure mode the typ1 entry exhibited: a magic
// listed in the dispatch but never actually parseable. If a future
// change adds a magic that ParseTTF / decodeWOFF / extractTTCFont
// cannot handle, this test surfaces it as a behavior regression.
func TestParseFontDispatchSurface(t *testing.T) {
	cases := []struct {
		name  string
		magic uint32
	}{
		{"TrueType (0x00010000)", 0x00010000},
		{"OpenType/CFF (\"OTTO\")", 0x4F54544F},
		{"legacy Apple TrueType (\"true\")", 0x74727565},
		{"WOFF1 (\"wOFF\")", woffMagic},
		{"TTC (\"ttcf\")", ttcMagic},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]byte, 16)
			binary.BigEndian.PutUint32(data[0:4], tc.magic)
			_, err := ParseFont(data)
			if err == nil {
				// No current parser returns nil for 16 magic-prefixed
				// zeroed bytes (sfnt rejects the truncated table
				// directory; decodeWOFF rejects on header size;
				// extractTTCFont rejects on numFonts == 0). Log
				// rather than fail so a future parser change that
				// accepts garbage at least surfaces in test output
				// without taking down the suite.
				t.Logf("unexpected nil error for magic 0x%08X (parser accepted minimal input)", tc.magic)
				return
			}
			if errors.Is(err, ErrUnknownFormat) {
				t.Errorf("magic 0x%08X is listed in dispatch but routed to ErrUnknownFormat: %v", tc.magic, err)
			}
		})
	}
}
