// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "golang.org/x/text/unicode/norm"

// normalizeText applies Unicode Normalization Form C (NFC) to user-supplied
// text at the layout entry point.
//
// Input that arrives in a decomposed form (for example U+0065 U+0301 for
// "e" followed by a combining acute accent) is canonically equivalent to
// its precomposed form (U+00E9). Font cmap tables, the shaping pipeline,
// and text-measurement routines in this package expect canonical composed
// input: some fonts only cover precomposed codepoints, and width
// accumulation over combining marks double-counts glyph advances that
// should be zero-width.
//
// Applying NFC once, at the boundary where user strings first become
// layout input, keeps the rest of the pipeline unaware of the difference
// between composed and decomposed inputs and produces byte-identical PDF
// output for canonically equivalent strings. NFC is idempotent: text that
// is already in NFC passes through unchanged, so this helper is safe to
// call repeatedly on the same value.
//
// See Unicode Standard Annex #15 (Unicode Normalization Forms) for the
// formal definition of NFC.
//
// Behavior on invalid UTF-8: norm.NFC passes ill-formed byte sequences
// through unchanged. This helper inherits that contract — invalid input
// does not panic and is not silently rewritten — so callers that hand
// us bytes from an external source (HTML, user paste, on-disk fixture)
// get the same bytes back when they were not valid UTF-8 to begin with.
func normalizeText(s string) string {
	if norm.NFC.IsNormalString(s) {
		return s
	}
	return norm.NFC.String(s)
}

// normalizeRuns returns a fresh []TextRun whose Text fields are
// NFC-normalized. The input slice is never mutated; callers can pass a
// caller-owned slice safely. Runs flagged as InlineElement or
// IsLineBreak are copied through with their Text untouched (they carry
// no user text that needs canonicalization).
func normalizeRuns(runs []TextRun) []TextRun {
	if len(runs) == 0 {
		return nil
	}
	out := make([]TextRun, len(runs))
	for i, r := range runs {
		out[i] = r
		if r.InlineElement != nil || r.IsLineBreak {
			continue
		}
		out[i].Text = normalizeText(r.Text)
	}
	return out
}
